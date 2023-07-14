// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package plan provide the Terraform planning functionality for Guardian.
package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/google/go-github/v53/github"
)

const planCommentPrefix = "**`🔱 Guardian 🔱 PLAN`** - "

// Result is the result of a plan operation.
type Result struct {
	hasChanges     bool
	commentDetails string
}

// Process handles the main logic for the Guardian plan process.
func (c *PlanCommand) Process(ctx context.Context) error {
	var merr error

	c.Outf("Starting Guardian plan")

	c.Outf("\n\nRemoving outdated comments...")
	if err := c.deleteOutdatedComments(ctx, c.cfg.RepositoryOwner, c.cfg.RepositoryName, c.cfg.PullRequestNumber); err != nil {
		return fmt.Errorf("failed to delete outdated comments: %w", err)
	}

	logURL := fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.cfg.RepositoryOwner, c.cfg.RepositoryName, c.cfg.RunID, c.cfg.RunAttempt)

	c.Outf("\n\nCreating start comment...")
	startComment, err := c.githubClient.CreateIssueComment(
		ctx,
		c.cfg.RepositoryOwner,
		c.cfg.RepositoryName,
		c.cfg.PullRequestNumber,
		fmt.Sprintf("%s 🟨 Running for dir: `%s` %s", planCommentPrefix, c.flagWorkingDirectory, logURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create start comment: %w", err)
	}

	c.Outf("\n\nRunning Terraform commands...")
	result, err := c.handleTerraformPlan(ctx)

	if err != nil {
		var comment strings.Builder

		comment.WriteString(fmt.Sprintf("%s 🟥 Failed for dir: `%s` %s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", planCommentPrefix, c.flagWorkingDirectory, logURL, err))
		if result.commentDetails != "" {
			comment.WriteString(fmt.Sprintf("\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails))
		}

		if _, commentErr := c.githubClient.CreateIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			c.cfg.PullRequestNumber,
			comment.String(),
		); commentErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to create plan error comment: %w", commentErr))
		}

		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian plan: %w", err))
	} else if result.hasChanges {
		var comment strings.Builder

		comment.WriteString(fmt.Sprintf("%s 🟩 Successful for dir: `%s` %s", planCommentPrefix, c.flagWorkingDirectory, logURL))
		if result.commentDetails != "" {
			comment.WriteString(fmt.Sprintf("\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails))
		}

		if _, commentErr := c.githubClient.CreateIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			c.cfg.PullRequestNumber,
			comment.String(),
		); commentErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to create plan comment: %w", commentErr))
		}
	}

	if !result.hasChanges {
		if err := c.githubClient.UpdateIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			startComment.GetID(),
			fmt.Sprintf("%s 🟦 No Terraform files have changes, planning skipped.", planCommentPrefix),
		); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to update plan comment: %w", err))
		}
	} else {
		if err := c.githubClient.DeleteIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			startComment.GetID(),
		); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to delete plan comment: %w", err))
		}
	}

	c.Outf("\n\nGuardian Plan completed")

	return merr
}

// handleTerraformPlan runs the required Terraform commands for a full run of
// a Guardian plan using the Terraform CLI.
func (c *PlanCommand) handleTerraformPlan(ctx context.Context) (*Result, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	lockfileMode := "none"
	if c.flagProtectLockfile {
		lockfileMode = "readonly"
	}

	c.actions.Group("Initializing Terraform")

	if _, err := c.terraformClient.Init(
		ctx,
		c.Stdout(),
		multiStderr,
		&terraform.InitOptions{
			Input:       util.Ptr[bool](false),
			NoColor:     util.Ptr[bool](true),
			Lockfile:    util.Ptr[string](lockfileMode),
			LockTimeout: util.Ptr[string](c.flagLockTimeout.String()),
		}); err != nil {
		return &Result{commentDetails: stderr.String()}, fmt.Errorf("failed to initialize: %w", err)
	}
	stderr.Reset()
	c.actions.EndGroup()

	c.actions.Group("Validating Terraform")

	if _, err := c.terraformClient.Validate(
		ctx,
		c.Stdout(),
		multiStderr,
		&terraform.ValidateOptions{NoColor: util.Ptr[bool](true)},
	); err != nil {
		return &Result{commentDetails: stderr.String()}, fmt.Errorf("failed to validate: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	c.actions.Group("Running Terraform plan")

	planExitCode, err := c.terraformClient.Plan(ctx, c.Stdout(), multiStderr, &terraform.PlanOptions{
		Out:              util.Ptr[string](c.planFilename),
		Input:            util.Ptr[bool](false),
		NoColor:          util.Ptr[bool](true),
		DetailedExitcode: util.Ptr[bool](true),
		LockTimeout:      util.Ptr[string](c.flagLockTimeout.String()),
	})

	stderr.Reset()
	c.actions.EndGroup()

	hasChanges := planExitCode == 2

	if err != nil && !hasChanges {
		return &Result{commentDetails: stderr.String()}, fmt.Errorf("failed to plan: %w", err)
	}

	if !hasChanges {
		return &Result{hasChanges: hasChanges}, nil
	}

	c.actions.Group("\n\nFormatting plan output")

	_, err = c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{File: util.Ptr[string](c.planFilename), NoColor: util.Ptr[bool](true)})
	if err != nil {
		return &Result{
			commentDetails: stderr.String(),
			hasChanges:     hasChanges,
		}, fmt.Errorf("failed to terraform show: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	githubOutput := terraform.FormatOutputForGitHubDiff(stdout.String())

	planFilePath := path.Join(c.flagWorkingDirectory, c.planFilename)

	planData, err := os.ReadFile(planFilePath)
	if err != nil {
		return &Result{
				hasChanges: hasChanges,
			},
			fmt.Errorf("failed to read plan binary: %w", err)
	}

	c.actions.Group("Uploading plan file")

	if err := c.handleUploadGuardianPlan(ctx, planFilePath, planData, planExitCode); err != nil {
		return &Result{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	return &Result{
		commentDetails: githubOutput,
		hasChanges:     hasChanges,
	}, nil
}

// handleUploadGuardianPlan uploads the Guardian plan binary to the configured Guardian storage bucket.
func (c *PlanCommand) handleUploadGuardianPlan(ctx context.Context, planFilePath string, planData []byte, planExitCode int) error {
	guardianPlanName := fmt.Sprintf("guardian-plans/%s/%s/%d/%s", c.cfg.RepositoryOwner, c.cfg.RepositoryName, c.cfg.PullRequestNumber, planFilePath)

	metadata := make(map[string]string)
	metadata["plan_exit_code"] = fmt.Sprintf("%d", planExitCode)

	if err := c.storageClient.UploadObject(ctx, c.flagBucketName, guardianPlanName, planData,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to upload plan file: %w", err)
	}

	return nil
}

// deleteOutdatedComments deletes the pull request comments from previous Guardian plan runs.
func (c *PlanCommand) deleteOutdatedComments(ctx context.Context, owner, repo string, number int) error {
	listOpts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := c.githubClient.ListIssueComments(ctx, owner, repo, number, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		for _, comment := range comments {
			if strings.HasPrefix(comment.GetBody(), planCommentPrefix) {
				if err := c.githubClient.DeleteIssueComment(ctx, owner, repo, comment.GetID()); err != nil {
					return fmt.Errorf("failed to delete comment: %w", err)
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return nil
}
