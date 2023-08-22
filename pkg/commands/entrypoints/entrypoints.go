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

// Package entrypoints provides the functionality to determine the entrypoints for Guardian.
package entrypoints

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/posener/complete/v2"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

var _ cli.Command = (*EntrypointsCommand)(nil)

// allowedFormats are the allowed format flags for this command.
var allowedFormats = map[string]struct{}{
	"json": {},
	"text": {},
}

// allowedFormatNames are the sorted allowed format names for the format flag.
// This is used for printing messages and prediction.
var allowedFormatNames = sortedMapKeys(allowedFormats)

type EntrypointsCommand struct {
	cli.BaseCommand

	directory string

	flags.GitHubFlags
	flags.RetryFlags

	flagPullRequestNumber       int
	flagDestRef                 string
	flagSourceRef               string
	flagSkipDetectChanges       bool
	flagFormat                  string
	flagFailUnresolvableModules bool

	gitClient git.Git
}

func (c *EntrypointsCommand) Desc() string {
	return `Determine the entrypoint directories to run Guardian commands`
}

func (c *EntrypointsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

	Determine the entrypoint directories to run Guardian commands.
`
}

func (c *EntrypointsCommand) Flags() *cli.FlagSet {
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

	f.StringVar(&cli.StringVar{
		Name:    "format",
		Target:  &c.flagFormat,
		Example: "json",
		Usage:   fmt.Sprintf("The format to print the output directories. The supported formats are: %s.", allowedFormatNames),
		Default: "text",
		Predict: complete.PredictFunc(func(prefix string) []string {
			return allowedFormatNames
		}),
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "fail-unresolvable-modules",
		Target:  &c.flagFailUnresolvableModules,
		Usage:   `Whether or not to error if a module cannot be resolved.`,
		Default: false,
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if !c.flagSkipDetectChanges && c.flagSourceRef == "" && c.flagDestRef == "" {
			merr = errors.Join(merr, fmt.Errorf("invalid flag: source-ref and dest-ref are required to detect changes, to ignore changes set the skip-detect-changes flag"))
		}

		if _, ok := allowedFormats[c.flagFormat]; !ok {
			merr = errors.Join(merr, fmt.Errorf("invalid flag: format %s (supported formats are: %s)", c.flagFormat, allowedFormatNames))
		}

		return merr
	})

	return set
}

func (c *EntrypointsCommand) Run(ctx context.Context, args []string) error {
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

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *EntrypointsCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner).
		With("pull_request_number", c.flagPullRequestNumber)

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
func (c *EntrypointsCommand) writeOutput(dirs []string) error {
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
		return fmt.Errorf("invalid format flag: %s (supported formats are: %s)", c.flagFormat, allowedFormatNames)
	}

	return nil
}

// sortedMapKeys returns the sorted slice of map key strings.
func sortedMapKeys(m map[string]struct{}) []string {
	k := maps.Keys(m)
	slices.Sort(k)
	return k
}
