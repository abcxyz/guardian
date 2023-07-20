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
	"flag"
	"fmt"
	"strings"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	gh "github.com/google/go-github/v53/github"
	"golang.org/x/exp/maps"
)

var _ cli.Command = (*PlanInitCommand)(nil)

// allowedFormats are the allowed format flags for this command.
var allowedFormats = map[string]struct{}{
	"json": {},
	"text": {},
}

type PlanInitCommand struct {
	cli.BaseCommand

	directory string

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber    int
	flagDestRef              string
	flagSourceRef            string
	flagKeepOutdatedComments bool
	flagSkipDetectChanges    bool
	flagFormat               string

	gitClient    git.Git
	githubClient github.GitHub

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option
}

func (c *PlanInitCommand) Desc() string {
	return `Run the Terraform plan for a directory`
}

func (c *PlanInitCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

  Initialize Guardian for running the Terraform plan process.
`
}

func (c *PlanInitCommand) Flags() *cli.FlagSet {
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

	f.BoolVar(&cli.BoolVar{
		Name:   "skip-detect-changes",
		Target: &c.flagSkipDetectChanges,
		Usage:  "Skip detecting file changes and run for all entrypoint directories.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:   "keep-outdated-comments",
		Target: &c.flagKeepOutdatedComments,
		Usage:  "Keep outdated comments when Guardian plan is run multiple times for the same pull request.",
	})

	f.StringVar(&cli.StringVar{
		Name:   "format",
		Target: &c.flagFormat,
		Usage:  fmt.Sprintf("The format to print the output directories. The supported formats are: %s.", strings.Join(maps.Keys(allowedFormats), ", ")),
	})

	return set
}

func (c *PlanInitCommand) Run(ctx context.Context, args []string) error {
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

	c.gitClient = git.NewGitClient(c.directory)
	c.githubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *PlanInitCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		Named("init.process").
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber)

	logger.Debug("starting Guardian plan init")

	if c.flagFormat == "" {
		c.flagFormat = "text"
	}

	if _, ok := allowedFormats[c.flagFormat]; !ok {
		return fmt.Errorf("invalid format flag: %s (supported formats are: %s)", c.flagFormat, strings.Join(maps.Keys(allowedFormats), ", "))
	}

	if c.GitHubFlags.FlagIsGitHubActions && !c.flagKeepOutdatedComments {
		logger.Debug("removing outdated comments...")
		if err := c.deleteOutdatedComments(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber); err != nil {
			return fmt.Errorf("failed to delete outdated comments: %w", err)
		}
	}

	logger.Debug("finding entrypoint directories")

	entrypointDirs, err := terraform.GetEntrypointDirectories(c.directory)
	if err != nil {
		return fmt.Errorf("failed to find terraform directories: %w", err)
	}
	logger.Debugw("terraform entrypoint directories", "entrypoint_dirs", entrypointDirs)

	if !c.flagSkipDetectChanges {
		logger.Debug("finding git diff directories")

		diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagDestRef, c.flagSourceRef)
		if err != nil {
			return fmt.Errorf("failed to find git diff directories: %w", err)
		}
		logger.Debugw("git diff directories", "directories", diffDirs)

		entrypointDirs = util.GetSliceIntersection(entrypointDirs, diffDirs)
	}

	logger.Debugw("target directories", "target_directories", entrypointDirs)

	if err := c.writeOutput(entrypointDirs); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// writeOutput writes the command output.
func (c *PlanInitCommand) writeOutput(dirs []string) error {
	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// convert to child path for output
	// using absolute path creates an ugly github workflow name
	for k, dir := range dirs {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for: %w", err)
		}
		dirs[k] = childPath
	}

	switch v := strings.TrimSpace(strings.ToLower(c.flagFormat)); v {
	case "json":
		if err := json.NewEncoder(c.Stdout()).Encode(dirs); err != nil {
			return fmt.Errorf("failed to create json string: %w", err)
		}
	case "text":
		for _, dir := range dirs {
			c.Outf("%s", dir)
		}
	default:
		return fmt.Errorf("invalid format flag: %s (supported formats are: %s)", c.flagFormat, strings.Join(maps.Keys(allowedFormats), ", "))
	}

	return nil
}

// deleteOutdatedComments deletes the pull request comments from previous Guardian plan runs.
func (c *PlanInitCommand) deleteOutdatedComments(ctx context.Context, owner, repo string, number int) error {
	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		response, err := c.githubClient.ListIssueComments(ctx, owner, repo, number, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Comments == nil {
			return nil
		}

		for _, comment := range response.Comments {
			if !strings.HasPrefix(comment.Body, planCommentPrefix) {
				continue
			}

			if err := c.githubClient.DeleteIssueComment(ctx, owner, repo, comment.ID); err != nil {
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
