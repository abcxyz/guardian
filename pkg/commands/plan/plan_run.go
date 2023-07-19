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
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/sethvargo/go-githubactions"
)

const planCommentPrefix = "**`🔱 Guardian 🔱 PLAN`** -"

var _ cli.Command = (*PlanRunCommand)(nil)

// RunResult is the result of a plan operation.
type RunResult struct {
	hasChanges     bool
	commentDetails string
}

type PlanRunCommand struct {
	cli.BaseCommand

	cfg *Config

	directory     string
	planChildPath string
	planFilename  string

	flags.GitHubFlags
	flags.RetryFlags

	flagBucketName           string
	flagPullRequestNumber    int
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	actions         *githubactions.Action
	githubClient    github.GitHub
	storageClient   storage.Storage
	terraformClient terraform.Terraform

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option
}

func (c *PlanRunCommand) Desc() string {
	return `Run the Terraform plan for a directory`
}

func (c *PlanRunCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <dir>

  Run the Terraform plan for a directory.
`
}

func (c *PlanRunCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	c.GitHubFlags.AddFlags(set)
	c.RetryFlags.AddFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.IntVar(&cli.IntVar{
		Name:    "pull-request-number",
		Target:  &c.flagPullRequestNumber,
		Example: "100",
		Usage:   "The GitHub pull request number associated with this plan run.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "bucket-name",
		Target:  &c.flagBucketName,
		Example: "my-guardian-state-bucket",
		Usage:   "The Google Cloud Storage bucket name to store Guardian plan files.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "allow-lockfile-changes",
		Target:  &c.flagAllowLockfileChanges,
		Default: true,
		Example: "true",
		Usage:   "Prevent modification of the Terraform lockfile.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "lock-timeout",
		Target:  &c.flagLockTimeout,
		Default: 10 * time.Minute,
		Example: "10m",
		Usage:   "The duration Terraform should wait to obtain a lock when running commands that modify state.",
	})

	return set
}

func (c *PlanRunCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) != 1 {
		return flag.ErrHelp
	}

	dirAbs, err := util.PathEvalAbs(parsedArgs[0])
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	planChildPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.planChildPath = planChildPath

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.actions.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &Config{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.Debugw("loaded configuration", "config", c.cfg)

	c.githubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)
	c.terraformClient = terraform.NewTerraformClient(c.directory)

	sc, err := storage.NewGoogleCloudStorage(
		ctx,
		storage.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		storage.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)
	if err != nil {
		return fmt.Errorf("failed to create google cloud storage client: %w", err)
	}
	c.storageClient = sc

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian plan run process.
func (c *PlanRunCommand) Process(ctx context.Context) error {
	var merr error

	c.Outf("Starting Guardian plan")

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	logURL := fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID, c.cfg.RunAttempt)

	c.Outf("Creating start comment")
	startComment, err := c.githubClient.CreateIssueComment(
		ctx,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		c.flagPullRequestNumber,
		fmt.Sprintf("%s 🟨 Running for dir: `%s` %s", planCommentPrefix, c.planChildPath, logURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create start comment: %w", err)
	}

	c.Outf("Running Terraform commands")
	result, err := c.handleTerraformPlan(ctx)
	if err != nil {
		var comment strings.Builder

		fmt.Fprintf(&comment, "%s 🟥 Failed for dir: `%s` %s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", planCommentPrefix, c.planChildPath, logURL, err)
		if result.commentDetails != "" {
			fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
		}

		if commentErr := c.githubClient.UpdateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
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

		fmt.Fprintf(&comment, "%s 🟩 Successful for dir: `%s` %s", planCommentPrefix, c.planChildPath, logURL)
		if result.commentDetails != "" {
			fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
		}

		if commentErr := c.githubClient.UpdateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
			startComment.ID,
			comment.String(),
		); commentErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to create plan comment: %w", commentErr))
		}

		return merr
	}

	if err := c.githubClient.UpdateIssueComment(
		ctx,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		startComment.ID,
		fmt.Sprintf("%s 🟦 No changes for dir: `%s` %s", planCommentPrefix, c.planChildPath, logURL),
	); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to update plan comment: %w", err))
	}

	return merr
}

// handleTerraformPlan runs the required Terraform commands for a full run of
// a Guardian plan using the Terraform CLI.
func (c *PlanRunCommand) handleTerraformPlan(ctx context.Context) (*RunResult, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	lockfileMode := "none"
	if !c.flagAllowLockfileChanges {
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
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to initialize: %w", err)
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
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to validate: %w", err)
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
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to plan: %w", err)
	}

	c.actions.Group("Formatting plan output")

	_, err = c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{File: util.Ptr(c.planFilename), NoColor: util.Ptr(true)})
	if err != nil {
		return &RunResult{
			commentDetails: stderr.String(),
			hasChanges:     hasChanges,
		}, fmt.Errorf("failed to terraform show: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	githubOutput := terraform.FormatOutputForGitHubDiff(stdout.String())

	planFilePath := path.Join(c.planChildPath, c.planFilename)

	planData, err := os.ReadFile(planFilePath)
	if err != nil {
		return &RunResult{
				hasChanges: hasChanges,
			},
			fmt.Errorf("failed to read plan binary: %w", err)
	}

	c.actions.Group("Uploading plan file")

	if err := c.handleUploadGuardianPlan(ctx, planFilePath, planData, planExitCode); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()
	c.actions.EndGroup()

	return &RunResult{
		commentDetails: githubOutput,
		hasChanges:     hasChanges,
	}, nil
}

// handleUploadGuardianPlan uploads the Guardian plan binary to the configured Guardian storage bucket.
func (c *PlanRunCommand) handleUploadGuardianPlan(ctx context.Context, planFilePath string, planData []byte, planExitCode int) error {
	guardianPlanName := fmt.Sprintf("guardian-plans/%s/%s/%d/%s", c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber, planFilePath)

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
