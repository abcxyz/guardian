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
	"strings"

	gh "github.com/google/go-github/v53/github"
	"github.com/posener/complete/v2"

	"github.com/abcxyz/guardian/pkg/commands/apply"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
)

var _ cli.Command = (*RemoveGuardianCommentsCommand)(nil)

// commandCommentPrefixes are the allowed commands comment prefixes that can be
// removed from a pull request.
var commandCommentPrefixes = map[string]string{
	"apply": apply.CommentPrefix,
	"plan":  plan.CommentPrefix,
}

// allowedCommands are the sorted allowed Guardian command names for the for-command flag.
// This is used for printing messages and prediction.
var allowedCommands = util.SortedMapKeys(commandCommentPrefixes)

type RemoveGuardianCommentsCommand struct {
	cli.BaseCommand

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber int
	flagForCommands       []string

	gitHubClient github.GitHub
}

func (c *RemoveGuardianCommentsCommand) Desc() string {
	return `Remove previous Guardian comments from a pull request`
}

func (c *RemoveGuardianCommentsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Remove previous Guardian comments from a pull request.
`
}

func (c *RemoveGuardianCommentsCommand) Flags() *cli.FlagSet {
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

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "for-command",
		Target:  &c.flagForCommands,
		Example: "plan",
		Usage:   fmt.Sprintf("The Guardian command to remove comments for. Valid values are %q", allowedCommands),
		Predict: complete.PredictFunc(func(prefix string) []string {
			return allowedCommands
		}),
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

		if len(c.flagForCommands) == 0 {
			merr = errors.Join(merr, fmt.Errorf("missing flag: for-command is required"))
		}

		unknown := sets.Subtract(c.flagForCommands, allowedCommands)
		if len(unknown) > 0 {
			merr = errors.Join(merr, fmt.Errorf("invalid value(s) for-command: %q must be one of %q", unknown, allowedCommands))
		}

		return merr
	})

	return set
}

func (c *RemoveGuardianCommentsCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

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

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian remove plan comments process.
func (c *RemoveGuardianCommentsCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber).
		With("for_commands", c.flagForCommands)

	logger.DebugContext(ctx, "removing outdated comments...")

	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		response, err := c.gitHubClient.ListIssueComments(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Comments == nil {
			return nil
		}

		for _, comment := range response.Comments {
			for _, commentType := range c.flagForCommands {
				prefix := commandCommentPrefixes[commentType]

				// prefix is not found, skip
				if !strings.HasPrefix(comment.Body, prefix) {
					continue
				}

				// found the prefix, delete the comment
				if err := c.gitHubClient.DeleteIssueComment(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, comment.ID); err != nil {
					return fmt.Errorf("failed to delete comment: %w", err)
				}

				// we deleted the comment, exit loop
				break
			}
		}

		if response.Pagination == nil {
			break
		}
		listOpts.Page = response.Pagination.NextPage
	}

	return nil
}
