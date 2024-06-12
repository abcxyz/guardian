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

package workflows

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/sethvargo/go-githubactions"
	"golang.org/x/exp/slices"

	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ cli.Command = (*PlanStatusCommentCommand)(nil)

type PlanStatusCommentCommand struct {
	cli.BaseCommand

	cfg *PlanStatusCommentsConfig

	gitHubLogURL string

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber int
	flagInitResult        string
	flagPlanResult        string

	actions      *githubactions.Action
	gitHubClient github.GitHub
}

func (c *PlanStatusCommentCommand) Desc() string {
	return `Remove previous Guardian plan comments from a pull request`
}

func (c *PlanStatusCommentCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Remove previous Guardian plan comments from a pull request.
`
}

func (c *PlanStatusCommentCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.IntVar(&cli.IntVar{
		Name:    "pull-request-number",
		Target:  &c.flagPullRequestNumber,
		Example: "100",
		Usage:   "The GitHub pull request number to remove plan comments from.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "init-result",
		Target:  &c.flagInitResult,
		Example: "success",
		Usage:   "The Guardian init job result status.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "plan-result",
		Target:  &c.flagPlanResult,
		Example: "failure",
		Usage:   "The Guardian plan job result status.",
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

		if c.flagInitResult == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: init-result is required"))
		}

		if c.flagPlanResult == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: plan-result is required"))
		}

		return merr
	})

	return set
}

func (c *PlanStatusCommentCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.actions.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &PlanStatusCommentsConfig{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.DebugContext(ctx, "loaded configuration", "plan_status_comments_config", c.cfg)

	tokenSource, err := c.GitHubFlags.TokenSource(map[string]string{
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

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian remove plan comments process.
func (c *PlanStatusCommentCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("init_result", c.flagInitResult).
		With("plan_result", c.flagPlanResult)

	logger.DebugContext(ctx, "determining plan status...")

	c.gitHubLogURL = fmt.Sprintf("[[logs](%s/%s/%s/actions/runs/%d/attempts/%d)]", c.cfg.ServerURL, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.RunID, c.cfg.RunAttempt)
	logger.DebugContext(ctx, "computed github log url", "github_log_url", c.gitHubLogURL)

	// this case improves user experience, when the planning job does not have a plan diff
	// there are no comments, this informs the user that the job ran successfully
	if c.flagInitResult == "success" && c.flagPlanResult == "success" {
		if _, err := c.gitHubClient.CreateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
			c.flagPullRequestNumber,
			fmt.Sprintf("%s 🟩 Plan completed successfully. %s", plan.CommentPrefix, c.gitHubLogURL),
		); err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}

		return nil
	}

	// this case does not require a comment because the planning job should
	// have commented that there was a failure, for which directory
	if c.flagInitResult == "failure" || c.flagPlanResult == "failure" {
		return fmt.Errorf("init or plan has one or more failures")
	}

	// this case improves user experience, when no Terraform changes were submitted
	// the plan job is skipped, this informs the user that the job ran successfully
	// but no changes are needed
	if c.flagInitResult == "success" && c.flagPlanResult == "skipped" {
		if _, err := c.gitHubClient.CreateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
			c.flagPullRequestNumber,
			fmt.Sprintf("%s 🟦 No Terraform changes detected, planning skipped. %s", plan.CommentPrefix, c.gitHubLogURL),
		); err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}

		return nil
	}

	indeterminateStatuses := []string{"skipped", "cancelled"}
	if slices.Contains(indeterminateStatuses, c.flagInitResult) || slices.Contains(indeterminateStatuses, c.flagPlanResult) {
		if _, err := c.gitHubClient.CreateIssueComment(
			ctx,
			c.GitHubFlags.FlagGitHubOwner,
			c.GitHubFlags.FlagGitHubRepo,
			c.flagPullRequestNumber,
			fmt.Sprintf("%s 🟨 Unable to determine plan status. %s", plan.CommentPrefix, c.gitHubLogURL),
		); err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}

		return fmt.Errorf("unable to determine plan status, init and/or plan was skipped or cancelled")
	}

	return nil
}
