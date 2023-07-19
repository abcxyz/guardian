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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	gh "github.com/google/go-github/v53/github"
)

var _ cli.Command = (*PlanInitCommand)(nil)

type PlanInitCommand struct {
	cli.BaseCommand

	workingDir  string
	entrypoints []string

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber      int
	flagDestRef                string
	flagSourceRef              string
	flagDirectories            []string
	flagDeleteOutdatedComments bool
	flagJSON                   bool

	gitClient    git.Git
	githubClient github.GitHub

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option
}

func (c *PlanInitCommand) Desc() string {
	return `Run the Terraform plan for a directory.`
}

func (c *PlanInitCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Initialize Guardian for running the Terraform plan process.
`
}

func (c *PlanInitCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	c.GitHubFlags.AddFlags(set)
	c.RetryFlags.AddFlags(set)

	f := set.NewSection("Command options")

	f.IntVar(&cli.IntVar{
		Name:    "pull-request-number",
		Target:  &c.flagPullRequestNumber,
		Example: "100",
		Usage:   "The GitHub pull request number associated with this plan run.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "dest-ref",
		Target:  &c.flagDestRef,
		Example: "ref-name",
		Usage:   "The destination GitHub ref name for finding file changes.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "source-ref",
		Target:  &c.flagSourceRef,
		Example: "ref-name",
		Usage:   "The source GitHub ref name for finding file changes.",
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "directories",
		Target:  &c.flagDirectories,
		Example: "dir1,dir2,dir3",
		Usage:   "The subset of directories to target for Terraform planning.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "delete-outdated-comments",
		Target:  &c.flagDeleteOutdatedComments,
		Default: true,
		Usage:   "Delete outdated comments when Guardian plan is run multiple times for the same pull request.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "json",
		Target:  &c.flagJSON,
		Default: false,
		Usage:   "Print the output directories in JSON format.",
	})

	return set
}

func (c *PlanInitCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	c.workingDir = cwd

	c.gitClient = git.NewGitClient(cwd)
	c.githubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithInitialRetryDelay(c.RetryFlags.FlagInitialRetryDelay),
		github.WithMaxRetries(c.RetryFlags.FlagMaxRetries),
		github.WithMaxRetryDelay(c.RetryFlags.FlagMaxRetryDelay),
	)

	if len(c.flagDirectories) == 0 {
		c.flagDirectories = append(c.flagDirectories, cwd)
	}

	logger.Debugln("finding entrypoint directories..")

	dirs, err := terraform.GetEntrypointDirectories(c.flagDirectories)
	if err != nil {
		return fmt.Errorf("failed to find terraform directories: %w", err)
	}

	c.entrypoints = dirs
	logger.Debugf("terraform entrypoint directories: %s", c.entrypoints)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian plan init process.
func (c *PlanInitCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Debugln("starting Guardian plan init")

	if c.GitHubFlags.FlagGitHubAction && c.flagDeleteOutdatedComments {
		logger.Debugln("removing outdated comments...")
		if err := c.deleteOutdatedComments(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber); err != nil {
			return fmt.Errorf("failed to delete outdated comments: %w", err)
		}
	}

	diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagDestRef, c.flagSourceRef)
	if err != nil {
		return fmt.Errorf("failed to find git diff directories: %w", err)
	}
	logger.Debugf("changed directories: %+v", diffDirs)

	targetDirs := util.GetSliceIntersection(c.entrypoints, diffDirs)
	logger.Debugf("target directories: %+v", targetDirs)

	// convert to child path for output, using absolute path
	// creates an ugly github workflow name
	for k, dir := range targetDirs {
		childPath, err := util.ChildPath(c.workingDir, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for: %w", err)
		}
		targetDirs[k] = childPath
	}

	if c.flagJSON {
		outJSON, err := json.Marshal(targetDirs)
		if err != nil {
			return fmt.Errorf("failed to create json string: %w", err)
		}
		c.Outf(string(outJSON))
	} else {
		for _, dir := range targetDirs {
			c.Outf("%s", dir)
		}
	}

	return nil
}

// deleteOutdatedComments deletes the pull request comments from previous Guardian plan runs.
func (c *PlanInitCommand) deleteOutdatedComments(ctx context.Context, owner, repo string, number int) error {
	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		comments, paging, err := c.githubClient.ListIssueComments(ctx, owner, repo, number, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		for _, comment := range comments {
			if strings.HasPrefix(comment.Body, planCommentPrefix) {
				if err := c.githubClient.DeleteIssueComment(ctx, owner, repo, comment.ID); err != nil {
					return fmt.Errorf("failed to delete comment: %w", err)
				}
			}
		}

		if !paging.HasNextPage {
			break
		}
		listOpts.Page = paging.NextPage
	}

	return nil
}
