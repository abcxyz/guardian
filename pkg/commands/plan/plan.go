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
	"strconv"
	"strings"

	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
)

const planCommentPrefix = "**`🔱 Guardian 🔱 PLAN`** -"

// Result is the result of a plan operation.
type Result struct {
	hasChanges     bool
	commentDetails string
}

// Process handles the main logic for the Guardian plan process.
func (c *PlanCommand) Process(ctx context.Context) error {
	var merr error

	c.Outf("Starting Guardian plan")

	logURL := fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.cfg.RepositoryOwner, c.cfg.RepositoryName, c.cfg.RunID, c.cfg.RunAttempt)

	c.Outf("Creating start comment...")
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

	c.Outf("Running Terraform commands...")
	result, err := c.handleTerraformPlan(ctx)
	if err != nil {
		var comment strings.Builder

		fmt.Fprintf(&comment, "%s 🟥 Failed for dir: `%s` %s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", planCommentPrefix, c.flagWorkingDirectory, logURL, err)
		if result.commentDetails != "" {
			fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
		}

		if commentErr := c.githubClient.UpdateIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			startComment.ID,
			comment.String(),
		); commentErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to create plan error comment: %w", commentErr))
		}

		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian plan: %w", err))

		return merr
	}

	if result.hasChanges {
		var comment strings.Builder

		fmt.Fprintf(&comment, "%s 🟩 Successful for dir: `%s` %s", planCommentPrefix, c.flagWorkingDirectory, logURL)
		if result.commentDetails != "" {
			fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
		}

		if commentErr := c.githubClient.UpdateIssueComment(
			ctx,
			c.cfg.RepositoryOwner,
			c.cfg.RepositoryName,
			startComment.ID,
			comment.String(),
		); commentErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to create plan comment: %w", commentErr))
		}

		return merr
	}

	if err := c.githubClient.UpdateIssueComment(
		ctx,
		c.cfg.RepositoryOwner,
		c.cfg.RepositoryName,
		startComment.ID,
		fmt.Sprintf("%s 🟦 No changes for dir: `%s` %s", planCommentPrefix, c.flagWorkingDirectory, logURL),
	); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to update plan comment: %w", err))
	}

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
			Input:       util.Ptr(false),
			NoColor:     util.Ptr(true),
			Lockfile:    util.Ptr(lockfileMode),
			LockTimeout: util.Ptr(c.flagLockTimeout.String()),
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
		&terraform.ValidateOptions{NoColor: util.Ptr(true)},
	); err != nil {
		return &Result{commentDetails: stderr.String()}, fmt.Errorf("failed to validate: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	c.actions.Group("Running Terraform plan")

	planExitCode, err := c.terraformClient.Plan(ctx, c.Stdout(), multiStderr, &terraform.PlanOptions{
		Out:              util.Ptr(c.planFilename),
		Input:            util.Ptr(false),
		NoColor:          util.Ptr(true),
		DetailedExitcode: util.Ptr(true),
		LockTimeout:      util.Ptr(c.flagLockTimeout.String()),
	})

	stderr.Reset()
	c.actions.EndGroup()

	// use the detailed exitcode from terraform to determine if there is a diff
	// 0 - success, no diff  1 - failed   2 - success, diff
	hasChanges := planExitCode == 2

	if err != nil && !hasChanges {
		return &Result{commentDetails: stderr.String()}, fmt.Errorf("failed to plan: %w", err)
	}

	if !hasChanges {
		return &Result{hasChanges: hasChanges}, nil
	}

	c.actions.Group("Formatting plan output")

	_, err = c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{File: util.Ptr(c.planFilename), NoColor: util.Ptr(true)})
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
	metadata["plan_exit_code"] = strconv.Itoa(planExitCode)

	if err := c.storageClient.UploadObject(ctx, c.flagBucketName, guardianPlanName, planData,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to upload plan file: %w", err)
	}

	return nil
}
