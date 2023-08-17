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

// Package initialize provides the initialization functionality for Guardian.
package initialize

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	gh "github.com/google/go-github/v53/github"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

var _ cli.Command = (*InitCommand)(nil)

// allowedFormats are the allowed format flags for this command.
var allowedFormats = map[string]struct{}{
	"json": {},
	"text": {},
}

type InitCommand struct {
	cli.BaseCommand

	cfg *Config

	directory string

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber          int
	flagDestRef                    string
	flagSourceRef                  string
	flagDeleteOutdatedPlanComments bool
	flagSkipDetectChanges          bool
	flagFormat                     string
	flagFailUnresolvableModules    bool
	flagRequiredPermissions        []string

	actions      *githubactions.Action
	gitClient    git.Git
	gitHubClient github.GitHub
}

func (c *InitCommand) Desc() string {
	return `Run initialization steps prior to running additional Guardian commands`
}

func (c *InitCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

	Run initialization steps prior to running additional Guardian commands.
`
}

func (c *InitCommand) Flags() *cli.FlagSet {
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
		Name:   "delete-outdated-plan-comments",
		Target: &c.flagDeleteOutdatedPlanComments,
		Usage:  "Delete outdated plan comments when Guardian plan is run multiple times for the same pull request.",
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "required-permissions",
		Target:  &c.flagRequiredPermissions,
		Example: "admin,write",
		Usage:   "The required set of permissions allowed to run these commands. One of admin, write, read, and none.",
	})

	f.StringVar(&cli.StringVar{
		Name:   "format",
		Target: &c.flagFormat,
		Usage:  fmt.Sprintf("The format to print the output directories. The supported formats are: %s.", strings.Join(maps.Keys(allowedFormats), ", ")),
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "fail-unresolvable-modules",
		Target:  &c.flagFailUnresolvableModules,
		Usage:   `Whether or not to error if a module cannot be resolved.`,
		Default: false,
	})

	return set
}

func (c *InitCommand) Run(ctx context.Context, args []string) error {
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

	c.gitClient = git.NewGitClient(c.directory)
	c.gitHubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *InitCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber)

	logger.DebugContext(ctx, "Starting Guardian init")

	if c.flagFormat == "" {
		c.flagFormat = "text"
	}

	if _, ok := allowedFormats[c.flagFormat]; !ok {
		return fmt.Errorf("invalid format flag: %s (supported formats are: %s)", c.flagFormat, strings.Join(maps.Keys(allowedFormats), ", "))
	}

	if len(c.flagRequiredPermissions) > 0 {
		logger.DebugContext(ctx, "checking required permissions")
		permission, err := c.gitHubClient.RepoUserPermissionLevel(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.Actor)
		if err != nil {
			return fmt.Errorf("failed to get repo permissions: %w", err)
		}

		if allowed := slices.Contains(c.flagRequiredPermissions, permission); !allowed {
			sort.Strings(c.flagRequiredPermissions)
			return fmt.Errorf("%s does not have the required permissions to run this command.\n\nRequired permissions are %q", c.cfg.Actor, c.flagRequiredPermissions)
		}
	}

	if c.flagDeleteOutdatedPlanComments {
		logger.DebugContext(ctx, "removing outdated comments...")
		if err := c.deleteOutdatedPlanComments(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.flagPullRequestNumber); err != nil {
			return fmt.Errorf("failed to delete outdated comments: %w", err)
		}
	}

	logger.DebugContext(ctx, "finding entrypoint directories")

	entrypoints, err := terraform.GetEntrypointDirectories(c.directory)
	if err != nil {
		return fmt.Errorf("failed to find terraform directories: %w", err)
	}

	entrypointDirs := make([]string, 0, len(entrypoints))
	for _, e := range entrypoints {
		entrypointDirs = append(entrypointDirs, e.Path)
	}

	logger.DebugContext(ctx, "terraform entrypoint directories", "entrypoint_dirs", entrypoints)

	if !c.flagSkipDetectChanges {
		logger.DebugContext(ctx, "finding git diff directories")

		diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagDestRef, c.flagSourceRef)
		if err != nil {
			return fmt.Errorf("failed to find git diff directories: %w", err)
		}
		logger.DebugContext(ctx, "git diff directories", "directories", diffDirs)

		moduleUsageGraph, err := terraform.ModuleUsage(ctx, c.directory, !c.flagFailUnresolvableModules)
		if err != nil {
			return fmt.Errorf("failed to get module usage for %s: %w", c.directory, err)
		}

		modifiedEntrypoints := make(map[string]struct{})

		for _, changedFile := range diffDirs {
			if entrypoints, ok := moduleUsageGraph.ModulesToEntrypoints[changedFile]; ok {
				for entrypoint := range entrypoints {
					modifiedEntrypoints[entrypoint] = struct{}{}
				}
			}
			if _, ok := moduleUsageGraph.EntrypointToModules[changedFile]; ok {
				modifiedEntrypoints[changedFile] = struct{}{}
			}
		}

		files := maps.Keys(modifiedEntrypoints)
		sort.Strings(files)

		entrypointDirs = files
	}

	logger.DebugContext(ctx, "target directories", "target_directories", entrypointDirs)

	if err := c.writeOutput(entrypointDirs); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// writeOutput writes the command output.
func (c *InitCommand) writeOutput(dirs []string) error {
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
func (c *InitCommand) deleteOutdatedPlanComments(ctx context.Context, owner, repo string, number int) error {
	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		response, err := c.gitHubClient.ListIssueComments(ctx, owner, repo, number, listOpts)
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

			if err := c.gitHubClient.DeleteIssueComment(ctx, owner, repo, comment.ID); err != nil {
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
