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
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/commands/actions"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/pointer"
)

const (
	CommentPrefix          = "**`🔱 Guardian 🔱 PLAN`** -"
	DestroyCommentText     = "\n**`🟧 DESTROY`**"
	gitHubMaxCommentLength = 65536

	// plan file metadata key representing the exit code.
	MetaKeyExitCode = "plan_exit_code"

	// plan file metadata key representing the operation (plan, destroy).
	MetaKeyOperation = "operation"

	// plan files metadata operation values.
	OperationPlan    = "plan"
	OperationDestroy = "destroy"
)

var _ cli.Command = (*PlanCommand)(nil)

// RunResult is the result of a plan operation.
type RunResult struct {
	hasChanges     bool
	commentDetails string
}

type PlanCommand struct {
	actions.GitHubActionCommand

	cfg *Config

	directory     string
	childPath     string
	planFilename  string
	gitHubLogURL  string
	storageParent string
	storagePrefix string

	flags.RetryFlags
	flags.CommonFlags
	flags.GitHubFlags

	flagDestroy              bool
	flagStorage              string
	flagJobName              string
	flagPullRequestNumber    int
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	gitHubClient    github.GitHub
	storageClient   storage.Storage
	terraformClient terraform.Terraform
}

func (c *PlanCommand) Desc() string {
	return `Run Terraform plan for a directory`
}

func (c *PlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Run Terraform plan for a directory.
`
}

func (c *PlanCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubActionCommand.Register(set)
	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)
	c.CommonFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.BoolVar(&cli.BoolVar{
		Name:    "destroy",
		Target:  &c.flagDestroy,
		Example: "true",
		Usage:   "Use the destroy flag to plan changes to destroy all infrastructure.",
	})

	f.IntVar(&cli.IntVar{
		Name:    "pull-request-number",
		Target:  &c.flagPullRequestNumber,
		Example: "100",
		Usage:   "The GitHub pull request number associated with this plan run.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "storage",
		Target:  &c.flagStorage,
		Example: "gcs://my-guardian-state-bucket",
		Usage:   "The storage strategy to store Guardian plan files (defaults to file://).",
	})

	f.StringVar(&cli.StringVar{
		Name:    "job-name",
		Target:  &c.flagJobName,
		Example: "plan (terraform/project1)",
		Usage:   "The Github Actions job name, used to generate the correct logs URL in PR comments.",
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
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if c.FlagDir == "" {
		c.FlagDir = cwd
	}

	if c.flagStorage == "" {
		c.flagStorage = path.Join("local:///", cwd)
	}

	dirAbs, err := util.PathEvalAbs(c.FlagDir)
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	childPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.childPath = childPath

	c.Action = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.Action.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &Config{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.DebugContext(ctx, "loaded configuration", "config", c.cfg)

	tokenSource, err := c.GitHubFlags.TokenSource(ctx, map[string]string{
		"contents":      "read",
		"pull_requests": "write",
	})
	if err != nil {
		return fmt.Errorf("failed to get token source: %w", err)
	}
	token, err := tokenSource.GitHubToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	c.gitHubClient = github.NewClient(
		ctx,
		token,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)
	c.terraformClient = terraform.NewTerraformClient(c.directory)

	sc, err := c.resolveStorageClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create google cloud storage client: %w", err)
	}
	c.storageClient = sc

	return c.Process(ctx)
}

// resolveStorageClient resolves and generated the storage client based on the storage flag.
func (c *PlanCommand) resolveStorageClient(ctx context.Context) (storage.Storage, error) {
	u, err := url.Parse(c.flagStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse storage flag url: %w", err)
	}

	t := u.Scheme

	c.storageParent = path.Join(u.Host, u.Path)

	if strings.EqualFold(t, storage.FilesystemType) {
		return storage.NewFilesystemStorage(ctx) //nolint:wrapcheck // Want passthrough
	}

	if strings.EqualFold(t, storage.GoogleCloudStorageType) {
		sc, err := storage.NewGoogleCloudStorage(ctx)
		if err != nil {
			return nil, err //nolint:wrapcheck // Want passthrough
		}

		c.storagePrefix = fmt.Sprintf("guardian-plans/%s/%s/%d", c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber)
		return sc, nil
	}

	return nil, fmt.Errorf("unknown storage type: %s", t)
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

	c.gitHubLogURL = fmt.Sprintf("[[logs](%s)]", c.resolveGitHubLogURL(ctx))
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

func (c *PlanCommand) resolveGitHubLogURL(ctx context.Context) string {
	logger := logging.FromContext(ctx)

	// Default to action summary page
	defaultLogURL := fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", c.cfg.ServerURL, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID, c.cfg.RunAttempt)

	if !c.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping github log url resolution", "is_github_action", c.FlagIsGitHubActions)
		return defaultLogURL
	}

	// Link to specific job's logs directly if possible
	directJobURL, err := c.gitHubClient.ResolveJobLogsURL(ctx, c.flagJobName, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID)
	if err != nil {
		logger.DebugContext(ctx, "could not resolve direct url to job logs", "err", err)
		return defaultLogURL
	}

	return directJobURL
}

func (c *PlanCommand) createStartCommentForActions(ctx context.Context) (*github.IssueComment, error) {
	logger := logging.FromContext(ctx)

	if !c.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping start comment", "is_github_action", c.FlagIsGitHubActions)
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

	if !c.FlagIsGitHubActions {
		logger.DebugContext(ctx, "skipping update result comment", "is_github_action", c.FlagIsGitHubActions)
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
	destroyText := ""
	if c.flagDestroy {
		destroyText = DestroyCommentText
	}

	msgBody := fmt.Sprintf("%s 🟦 No changes for dir: `%s` %s%s", CommentPrefix, c.childPath, c.gitHubLogURL, destroyText)

	if result.hasChanges || resultErr != nil {
		var comment strings.Builder
		if resultErr != nil {
			fmt.Fprintf(&comment, "%s 🟥 Failed for dir: `%s` %s%s\n\n<details>\n<summary>Error</summary>\n\n```\n\n%s\n```\n</details>", CommentPrefix, c.childPath, c.gitHubLogURL, destroyText, resultErr)
		} else if result.hasChanges {
			fmt.Fprintf(&comment, "%s 🟩 Successful for dir: `%s` %s%s", CommentPrefix, c.childPath, c.gitHubLogURL, destroyText)
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

	if err := c.WithActionsOutGroup("Check Terraform Format", func() error {
		_, err := c.terraformClient.Format(ctx, multiStdout, multiStderr, &terraform.FormatOptions{
			Check:     pointer.To(true),
			Diff:      pointer.To(true),
			Recursive: pointer.To(true),
			NoColor:   pointer.To(true),
		})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to check formatting: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	lockfileMode := "none"
	if !c.flagAllowLockfileChanges {
		lockfileMode = "readonly"
	}

	if err := c.WithActionsOutGroup("Initializing Terraform", func() error {
		_, err := c.terraformClient.Init(ctx, multiStdout, multiStderr, &terraform.InitOptions{
			Input:       pointer.To(false),
			NoColor:     pointer.To(true),
			Lockfile:    pointer.To(lockfileMode),
			LockTimeout: pointer.To(c.flagLockTimeout.String()),
		})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to initialize: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := c.WithActionsOutGroup("Validating Terraform", func() error {
		_, err := c.terraformClient.Validate(ctx, multiStdout, multiStderr, &terraform.ValidateOptions{NoColor: pointer.To(true)})
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to validate: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	var hasChanges bool
	var planExitCode int

	if err := c.WithActionsOutGroup("Planning Terraform", func() error {
		exitCode, err := c.terraformClient.Plan(ctx, multiStdout, multiStderr, &terraform.PlanOptions{
			Out:              pointer.To(c.planFilename),
			Input:            pointer.To(false),
			NoColor:          pointer.To(true),
			Destroy:          pointer.To(c.flagDestroy),
			DetailedExitcode: pointer.To(true),
			LockTimeout:      pointer.To(c.flagLockTimeout.String()),
		})

		planExitCode = exitCode
		// use the detailed exitcode from terraform to determine if there is a diff
		// 0 - success, no diff  1 - failed   2 - success, diff
		hasChanges = planExitCode == 2

		return err //nolint:wrapcheck // Want passthrough
	}); err != nil && !hasChanges {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to plan: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	if err := c.WithActionsOutGroup("Formatting output", func() error {
		_, err := c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{
			File:    pointer.To(c.planFilename),
			NoColor: pointer.To(true),
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

	planFileLocalPath := path.Join(c.childPath, c.planFilename)

	planData, err := os.ReadFile(planFileLocalPath)
	if err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to read plan binary: %w", err)
	}

	planStoragePath := path.Join(c.storagePrefix, planFileLocalPath)

	c.Outf("Saving Plan File: %s", path.Join(c.storageParent, planStoragePath))

	if err := c.uploadGuardianPlan(ctx, planStoragePath, planData, planExitCode); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()

	return &RunResult{
		commentDetails: githubOutput,
		hasChanges:     hasChanges,
	}, nil
}

// uploadGuardianPlan uploads the Guardian plan binary to the configured Guardian storage bucket.
func (c *PlanCommand) uploadGuardianPlan(ctx context.Context, path string, data []byte, exitCode int) error {
	metadata := make(map[string]string)
	metadata[MetaKeyExitCode] = strconv.Itoa(exitCode)

	metadata[MetaKeyOperation] = OperationPlan
	if c.flagDestroy {
		metadata[MetaKeyOperation] = OperationDestroy
	}

	if err := c.storageClient.CreateObject(ctx, c.storageParent, path, data,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to upload plan file: %w", err)
	}

	return nil
}
