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

	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

const (
	CommentPrefix          = "**`🔱 Guardian 🔱 PLAN`** -"
	gitHubMaxCommentLength = 65536
)

var _ cli.Command = (*PlanCommand)(nil)

// RunResult is the result of a plan operation.
type RunResult struct {
	hasChanges     bool
	commentDetails string
}

type PlanCommand struct {
	cli.BaseCommand

	cfg *Config

	directory    string
	childPath    string
	planFilename string
	gitHubLogURL string

	flags.GitHubFlags
	flags.RetryFlags

	flagBucketName           string
	flagPullRequestNumber    int
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	actions         *githubactions.Action
	gitHubClient    github.GitHub
	storageClient   storage.Storage
	terraformClient terraform.Terraform
}

func (c *PlanCommand) Desc() string {
	return `Run Terraform plan for a directory`
}

func (c *PlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

  Run Terraform plan for a directory.
`
}

func (c *PlanCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)

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
		Example: "true",
		Usage:   "Allow modification of the Terraform lockfile.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "lock-timeout",
		Target:  &c.flagLockTimeout,
		Default: 10 * time.Minute,
		Example: "10m",
		Usage:   "The duration Terraform should wait to obtain a lock when running commands that modify state.",
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.GitHubFlags.FlagGitHubOwner == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: github-owner is required"))
		}

		if c.GitHubFlags.FlagGitHubRepo == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: github-repo is required"))
		}

		if c.flagPullRequestNumber <= 0 {
			merr = errors.Join(merr, fmt.Errorf("missing flag: pull-request-number is required"))
		}

		if c.flagBucketName == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: bucket-name is required"))
		}

		return merr
	})

	return set
}

func (c *PlanCommand) Run(ctx context.Context, args []string) error {
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

	childPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.childPath = childPath

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.actions.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &Config{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.DebugContext(ctx, "loaded configuration", "config", c.cfg)

	c.gitHubClient = github.NewClient(
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
func (c *PlanCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber)

	var merr error

	c.Outf("Starting Guardian plan")

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	c.gitHubLogURL = fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID, c.cfg.RunAttempt)
	logger.DebugContext(ctx, "computed github log url", "github_log_url", c.gitHubLogURL)

	startComment, err := c.createStartCommentForActions(ctx)
	if err != nil {
		return fmt.Errorf("failed to write start comment: %w", err)
	}

	result, err := c.terraformPlan(ctx)
	if err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian plan: %w", err))
	}

	if err := c.updateResultCommentForActions(ctx, startComment, result, err); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to write result comment: %w", err))
	}

	return merr
}

func (c *PlanCommand) createStartCommentForActions(ctx context.Context) (*github.IssueComment, error) {
	logger := logging.FromContext(ctx)

	if !c.GitHubFlags.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping start comment", "is_github_action", c.GitHubFlags.FlagIsGitHubActions)
		return nil, nil
	}

	c.Outf("Creating start comment")

	startComment, err := c.gitHubClient.CreateIssueComment(
		ctx,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		c.flagPullRequestNumber,
		fmt.Sprintf("%s 🟨 Running for dir: `%s` %s", CommentPrefix, c.childPath, c.gitHubLogURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create start comment: %w", err)
	}

	return startComment, nil
}

func (c *PlanCommand) updateResultCommentForActions(ctx context.Context, startComment *github.IssueComment, result *RunResult, resultErr error) error {
	logger := logging.FromContext(ctx)

	if !c.GitHubFlags.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping update result comment", "is_github_action", c.GitHubFlags.FlagIsGitHubActions)
		return nil
	}

	c.Outf("Updating result comment")
	msgBody := c.getMessageBody(result, resultErr)

	if err := c.gitHubClient.UpdateIssueComment(
		ctx,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		startComment.ID,
		msgBody,
	); err != nil {
		return fmt.Errorf("failed to update plan comment: %w", err)
	}

	return nil
}

func (c *PlanCommand) getMessageBody(result *RunResult, resultErr error) string {
	msgBody := fmt.Sprintf("%s 🟦 No changes for dir: `%s` %s", CommentPrefix, c.childPath, c.gitHubLogURL)

	if result.hasChanges || resultErr != nil {
		var comment strings.Builder
		if resultErr != nil {
			fmt.Fprintf(&comment, "%s 🟥 Failed for dir: `%s` %s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", CommentPrefix, c.childPath, c.gitHubLogURL, resultErr)
		} else if result.hasChanges {
			fmt.Fprintf(&comment, "%s 🟩 Successful for dir: `%s` %s", CommentPrefix, c.childPath, c.gitHubLogURL)
		}
		if result.commentDetails != "" {
			// Ensure the comment is not over GitHub's limit. We need to account for the surrounding characters we will
			// be adding in addition to the length of result.commentDetails.
			fmtString := "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>"
			truncationMsg := []rune("\n\nMessage has been truncated. See workflow logs to view the full message.")
			ellipses := []rune("...")
			cappedLength := gitHubMaxCommentLength - len(ellipses) - len(truncationMsg) - len([]rune(comment.String())) - len([]rune(fmtString)) + 2
			truncated := false
			if len([]rune(result.commentDetails)) > cappedLength {
				runes := []rune(result.commentDetails)[:cappedLength]
				runes = append(runes, ellipses...)
				result.commentDetails = string(runes)
				truncated = true
			}
			fmt.Fprintf(&comment, fmtString, result.commentDetails)
			if truncated {
				fmt.Fprint(&comment, string(truncationMsg))
			}
		}
		msgBody = comment.String()
	}

	return msgBody
}

// terraformPlan runs the required Terraform commands for a full run of
// a Guardian plan using the Terraform CLI.
func (c *PlanCommand) terraformPlan(ctx context.Context) (*RunResult, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	c.Outf("Running Terraform commands")

	if err := c.withActionsOutGroup("Check Terraform Format", func() error {
		_, err := c.terraformClient.Format(ctx, c.Stdout(), multiStderr, &terraform.FormatOptions{
			Check:     util.Ptr(true),
			Diff:      util.Ptr(true),
			Recursive: util.Ptr(true),
			NoColor:   util.Ptr(true),
		})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to check formatting: %w", err)
	}

	stderr.Reset()

	lockfileMode := "none"
	if !c.flagAllowLockfileChanges {
		lockfileMode = "readonly"
	}

	if err := c.withActionsOutGroup("Initializing Terraform", func() error {
		_, err := c.terraformClient.Init(ctx, c.Stdout(), multiStderr, &terraform.InitOptions{
			Input:       util.Ptr(false),
			NoColor:     util.Ptr(true),
			Lockfile:    util.Ptr(lockfileMode),
			LockTimeout: util.Ptr(c.flagLockTimeout.String()),
		})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to initialize: %w", err)
	}

	stderr.Reset()

	if err := c.withActionsOutGroup("Validating Terraform", func() error {
		_, err := c.terraformClient.Validate(ctx, c.Stdout(), multiStderr, &terraform.ValidateOptions{NoColor: util.Ptr(true)})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to validate: %w", err)
	}

	stderr.Reset()

	var hasChanges bool
	var planExitCode int

	if err := c.withActionsOutGroup("Planning Terraform", func() error {
		exitCode, err := c.terraformClient.Plan(ctx, c.Stdout(), multiStderr, &terraform.PlanOptions{
			Out:              util.Ptr(c.planFilename),
			Input:            util.Ptr(false),
			NoColor:          util.Ptr(true),
			DetailedExitcode: util.Ptr(true),
			LockTimeout:      util.Ptr(c.flagLockTimeout.String()),
		})

		planExitCode = exitCode
		// use the detailed exitcode from terraform to determine if there is a diff
		// 0 - success, no diff  1 - failed   2 - success, diff
		hasChanges = planExitCode == 2

		return err //nolint:wrapcheck // Want passthrough
	}); err != nil && !hasChanges {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to plan: %w", err)
	}

	stderr.Reset()

	if err := c.withActionsOutGroup("Formatting output", func() error {
		_, err := c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{
			File:    util.Ptr(c.planFilename),
			NoColor: util.Ptr(true),
		})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return &RunResult{
			commentDetails: stderr.String(),
			hasChanges:     hasChanges,
		}, fmt.Errorf("failed to terraform show: %w", err)
	}

	stderr.Reset()

	githubOutput := terraform.FormatOutputForGitHubDiff(stdout.String())

	planFilePath := path.Join(c.childPath, c.planFilename)

	planData, err := os.ReadFile(planFilePath)
	if err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to read plan binary: %w", err)
	}

	bucketObjectPath := fmt.Sprintf("guardian-plans/%s/%s/%d/%s", c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber, planFilePath)
	c.Outf("Uploading plan file to gs://%s/%s", c.flagBucketName, bucketObjectPath)

	if err := c.uploadGuardianPlan(ctx, bucketObjectPath, planData, planExitCode); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()

	return &RunResult{
		commentDetails: githubOutput,
		hasChanges:     hasChanges,
	}, nil
}

// withActionsOutGroup runs a function and ensures it is wrapped in GitHub actions
// grouping syntax. If this is not in an action, output is printed without grouping syntax.
func (c *PlanCommand) withActionsOutGroup(msg string, fn func() error) error {
	if c.GitHubFlags.FlagIsGitHubActions {
		c.actions.Group(msg)
		defer c.actions.EndGroup()
	} else {
		c.Outf(msg)
	}

	return fn()
}

// uploadGuardianPlan uploads the Guardian plan binary to the configured Guardian storage bucket.
func (c *PlanCommand) uploadGuardianPlan(ctx context.Context, path string, data []byte, exitCode int) error {
	metadata := make(map[string]string)
	metadata["plan_exit_code"] = strconv.Itoa(exitCode)

	if err := c.storageClient.UploadObject(ctx, c.flagBucketName, path, data,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to upload plan file: %w", err)
	}

	return nil
}
