// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package apply provides the Terraform apply functionality for Guardian.
package apply

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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

const (
	ownerReadWritePerms = 0o600
	CommentPrefix       = "**`🔱 Guardian 🔱 APPLY`** -"
)

var _ cli.Command = (*ApplyRunCommand)(nil)

// RunResult is the result of a apply operation.
type RunResult struct {
	commentDetails string
}

// ApplyRunCommand is a subcommand of apply and implements the cli.Command interface.
// It performs terraform apply on the given working directory.
type ApplyRunCommand struct {
	cli.BaseCommand

	cfg *Config

	directory                 string
	childPath                 string
	planFileName              string
	planFilePath              string
	gitHubLogURL              string
	computedPullRequestNumber int

	flags.GitHubFlags
	flags.RetryFlags

	flagBucketName           string
	flagCommitSHA            string
	flagPullRequestNumber    int
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	actions         *githubactions.Action
	gitHubClient    github.GitHub
	storageClient   storage.Storage
	terraformClient terraform.Terraform
}

// Desc provides a short, one-line description of the command.
func (c *ApplyRunCommand) Desc() string {
	return "Run Terraform apply for a directory"
}

// Help is the long-form help output to include usage instructions and flag
// information.
func (c *ApplyRunCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

	Run Terraform apply for a directory.
`
}

func (c *ApplyRunCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.IntVar(&cli.IntVar{
		Name:    "pull-request-number",
		Target:  &c.flagPullRequestNumber,
		Example: "100",
		Usage:   "The GitHub pull request number associated with this apply run. Only one of pull-request-number and commit-sha can be given.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "commit-sha",
		Target:  &c.flagCommitSHA,
		Example: "e538db9a29f2ff7a404a2ef40bb62a6df88c98c1",
		Usage:   "The commit sha to determine the pull request number associated with this apply run. Only one of pull-request-number and commit-sha can be given.",
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
		Usage:   "Allow modification of the Terraform lockfile.",
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

func (c *ApplyRunCommand) Run(ctx context.Context, args []string) error {
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

// Process handles the main logic for the Guardian apply run process.
func (c *ApplyRunCommand) Process(ctx context.Context) (merr error) {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("commit_sha", c.flagCommitSHA).
		With("pull_request_number", c.flagPullRequestNumber)

	c.Outf("Starting Guardian apply")

	if c.flagCommitSHA != "" && c.flagPullRequestNumber > 0 {
		return errors.New("only one of pull-request-number and commit-sha are allowed")
	}

	if c.planFileName == "" {
		c.planFileName = "tfplan.binary"
	}

	if c.flagCommitSHA != "" {
		prResponse, err := c.gitHubClient.ListPullRequestsForCommit(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagCommitSHA, nil)
		if err != nil {
			return fmt.Errorf("failed to get pull request number for commit sha: %w", err)
		}

		if prResponse.PullRequests == nil {
			return fmt.Errorf("no pull requests found for commit sha")
		}

		c.computedPullRequestNumber = prResponse.PullRequests[0].Number
	}

	if c.flagPullRequestNumber > 0 {
		c.computedPullRequestNumber = c.flagPullRequestNumber
	}
	logger.DebugContext(ctx, "computed pull request number", "computed_pull_request_number", c.computedPullRequestNumber)

	c.gitHubLogURL = fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID, c.cfg.RunAttempt)
	logger.DebugContext(ctx, "computed github log url", "github_log_url", c.gitHubLogURL)

	planBucketPath := path.Join(c.childPath, c.planFileName)
	bucketObjectPath := fmt.Sprintf("guardian-plans/%s/%s/%d/%s", c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.computedPullRequestNumber, planBucketPath)
	logger.DebugContext(ctx, "bucket object path", "bucket_object_path", bucketObjectPath)

	planData, planExitCode, err := c.downloadGuardianPlan(ctx, bucketObjectPath)
	if err != nil {
		return fmt.Errorf("failed to download guardian plan file: %w", err)
	}

	// we always want to delete the remote plan file to keep the bucket clean
	defer func() {
		if err := c.deleteGuardianPlan(ctx, bucketObjectPath); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to delete plan file: %w", err))
		}
	}()

	// exit code of 0 means success with no diff, skip apply
	if planExitCode == "0" {
		logger.DebugContext(ctx, "plan file has no diff, exiting", "plan_exit_code", planExitCode)
		c.Outf("Guardian plan file has no diff, exiting")
		return
	}

	tempDir, err := os.MkdirTemp("", "guardian-plans-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary plan directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to delete temporary plan directory: %w", err))
		}
	}()

	c.Outf("Writing plan file to disk")

	planFilePath := filepath.Join(tempDir, c.planFileName)
	if err := os.WriteFile(planFilePath, planData, ownerReadWritePerms); err != nil {
		return fmt.Errorf("failed to write plan file to disk [%s]: %w", planFilePath, err)
	}
	c.planFilePath = planFilePath

	startComment, err := c.createStartCommentForActions(ctx)
	if err != nil {
		return fmt.Errorf("failed to write start comment: %w", err)
	}

	result, err := c.terraformApply(ctx)
	if err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian apply: %w", err))
	}

	if err := c.updateResultCommentForActions(ctx, startComment, result, err); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to write result comment: %w", err))
	}

	return merr
}

func (c *ApplyRunCommand) createStartCommentForActions(ctx context.Context) (*github.IssueComment, error) {
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
		c.computedPullRequestNumber,
		fmt.Sprintf("%s 🟨 Running for dir: `%s` %s", CommentPrefix, c.childPath, c.gitHubLogURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create start comment: %w", err)
	}

	return startComment, nil
}

func (c *ApplyRunCommand) updateResultCommentForActions(ctx context.Context, startComment *github.IssueComment, result *RunResult, resulErr error) error {
	logger := logging.FromContext(ctx)

	if !c.GitHubFlags.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping update result comment", "is_github_action", c.GitHubFlags.FlagIsGitHubActions)
		return nil
	}

	c.Outf("Updating result comment")

	if resulErr != nil {
		var comment strings.Builder

		fmt.Fprintf(&comment, "%s 🟥 Failed for dir: `%s` %s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", CommentPrefix, c.childPath, c.gitHubLogURL, resulErr)
		if result.commentDetails != "" {
			fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
		}

		if err := c.gitHubClient.UpdateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
			startComment.ID,
			comment.String(),
		); err != nil {
			return fmt.Errorf("failed to update apply error comment: %w", err)
		}

		return nil
	}

	var comment strings.Builder

	fmt.Fprintf(&comment, "%s 🟩 Successful for dir: `%s` %s", CommentPrefix, c.childPath, c.gitHubLogURL)
	if result.commentDetails != "" {
		fmt.Fprintf(&comment, "\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s\n```\n</details>", result.commentDetails)
	}

	if commentErr := c.gitHubClient.UpdateIssueComment(
		ctx,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		startComment.ID,
		comment.String(),
	); commentErr != nil {
		return fmt.Errorf("failed to create apply comment: %w", commentErr)
	}

	return nil
}

// terraformApply runs the required Terraform commands for a full run of
// a Guardian apply using the Terraform CLI.
func (c *ApplyRunCommand) terraformApply(ctx context.Context) (*RunResult, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	c.Outf("Running Terraform commands")

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

	if err := c.withActionsOutGroup("Applying Terraform", func() error {
		_, err := c.terraformClient.Apply(ctx, multiStdout, multiStderr, &terraform.ApplyOptions{
			File:            util.Ptr(c.planFilePath),
			CompactWarnings: util.Ptr(true),
			Input:           util.Ptr(false),
			NoColor:         util.Ptr(true),
			LockTimeout:     util.Ptr(c.flagLockTimeout.String()),
		})

		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to apply: %w", err)
	}

	stderr.Reset()

	c.Outf("Formatting output")
	githubOutput := terraform.FormatOutputForGitHubDiff(stdout.String())

	return &RunResult{commentDetails: githubOutput}, nil
}

// withActionsOutGroup runs a function and ensures it is wrapped in GitHub actions
// grouping syntax. If this is not in an action, output is printed without grouping syntax.
func (c *ApplyRunCommand) withActionsOutGroup(msg string, fn func() error) error {
	if c.GitHubFlags.FlagIsGitHubActions {
		c.actions.Group(msg)
		defer c.actions.EndGroup()
	} else {
		c.Outf(msg)
	}

	return fn()
}

// downloadGuardianPlan downloads the Guardian plan binary from the configured Guardian storage bucket
// and returns the plan data and plan exit code.
func (c *ApplyRunCommand) downloadGuardianPlan(ctx context.Context, path string) (planData []byte, planExitCode string, outErr error) {
	c.Outf("Downloading Guardian plan file")

	metadata, err := c.storageClient.ObjectMetadata(ctx, c.flagBucketName, path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get plan object metadata: %w", err)
	}

	exitCode, ok := metadata["plan_exit_code"]
	if !ok {
		return nil, "", fmt.Errorf("failed to determine plan exit code: %w", err)
	}
	planExitCode = exitCode

	rc, err := c.storageClient.DownloadObject(ctx, c.flagBucketName, path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download object: %w", err)
	}

	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			outErr = fmt.Errorf("failed to close download object reader: %w", closeErr)
		}
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read plan data: %w", err)
	}
	planData = data

	return planData, planExitCode, nil
}

// handleDeleteGuardianPlan deletes the Guardian plan binary from the configured Guardian storage bucket.
func (c *ApplyRunCommand) deleteGuardianPlan(ctx context.Context, path string) error {
	c.Outf("Deleting Guardian plan file")

	if err := c.storageClient.DeleteObject(ctx, c.flagBucketName, path); err != nil {
		return fmt.Errorf("failed to delete apply file: %w", err)
	}

	return nil
}
