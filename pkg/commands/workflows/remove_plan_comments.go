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

// Package provides worklow helper functionality for Guardian.
package workflows

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	gh "github.com/google/go-github/v53/github"
)

var _ cli.Command = (*RemovePlanCommentsCommand)(nil)

type RemovePlanCommentsCommand struct {
	cli.BaseCommand

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber int

	gitHubClient github.GitHub
}

func (c *RemovePlanCommentsCommand) Desc() string {
	return `Remove previous Guardian plan comments from a pull request`
}

func (c *RemovePlanCommentsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <pull_request_number>

	Remove previous Guardian plan comments from a pull request.
`
}

func (c *RemovePlanCommentsCommand) Flags() *cli.FlagSet {
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

func (c *RemovePlanCommentsCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	c.gitHubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian remove plan comments process.
func (c *RemovePlanCommentsCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber)

	logger.DebugContext(ctx, "removing outdated plan comments...")

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
			if !strings.HasPrefix(comment.Body, plan.CommentPrefix) {
				continue
			}

			if err := c.gitHubClient.DeleteIssueComment(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, comment.ID); err != nil {
				return fmt.Errorf("failed to delete comment: %w", err)
			}
		}

		if response.Pagination == nil {
			break
		}
		listOpts.Page = response.Pagination.NextPage
	}

	return nil
}
