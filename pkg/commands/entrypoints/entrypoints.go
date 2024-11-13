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
	"io/fs"
	"slices"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ cli.Command = (*EntrypointsCommand)(nil)

type EntrypointsCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	flagDir                     []string
	flagDestRef                 string
	flagSourceRef               string
	flagDetectChanges           bool
	flagFailUnresolvableModules bool
	flagMaxDepth                int

	parsedFlagMaxDepth *int

	platformClient platform.Platform
	reporterClient reporter.Reporter

	newGitClient func(ctx context.Context, dir string) git.Git
}

func (c *EntrypointsCommand) Desc() string {
	return `Determine the entrypoint directories to run Guardian commands`
}

func (c *EntrypointsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Determine the entrypoint directories to run Guardian commands.
`
}

func (c *EntrypointsCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	f := set.NewSection("COMMAND OPTIONS")

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "dir",
		Target:  &c.flagDir,
		Example: "./terraform",
		Usage:   "The location of the terraform directory to search in. This flag can be repeated. Defaults to the current working directory.",
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
		Name:   "detect-changes",
		Target: &c.flagDetectChanges,
		Usage:  "Detect file changes, including all local module dependencies, and run for all entrypoint directories.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "fail-unresolvable-modules",
		Target:  &c.flagFailUnresolvableModules,
		Usage:   `Whether or not to error if a module cannot be resolved.`,
		Default: false,
	})

	f.IntVar(&cli.IntVar{
		Name:    "max-depth",
		Target:  &c.flagMaxDepth,
		Usage:   `How far to traverse the filesystem beneath the target directory for entrypoints.`,
		Default: -1,
	})

	// should come after command options in help output
	c.platformConfig.RegisterFlags(set)

	set.AfterParse(func(existingErr error) (merr error) {
		if c.flagDetectChanges && c.flagSourceRef == "" && c.flagDestRef == "" {
			merr = errors.Join(merr, fmt.Errorf("invalid flag: source-ref and dest-ref are required to detect changes, to ignore changes set the detect-changes flag"))
		}

		if c.flagMaxDepth != -1 {
			c.parsedFlagMaxDepth = &c.flagMaxDepth
		}

		return merr
	})

	return set
}

func (c *EntrypointsCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_entrypoints", 1)

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

	if len(c.flagDir) == 0 {
		c.flagDir = append(c.flagDir, cwd)
	}

	if c.newGitClient == nil {
		c.newGitClient = func(ctx context.Context, dir string) git.Git {
			return git.NewGitClient(dir)
		}
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platformClient = platform

	rc, err := reporter.NewReporter(ctx, c.platformConfig.Reporter, &reporter.Config{GitHub: c.platformConfig.GitHub})
	if err != nil {
		return fmt.Errorf("failed to create reporter client: %w", err)
	}
	c.reporterClient = rc

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *EntrypointsCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "starting entrypoints",
		"platform", c.platformConfig.Type,
		"reporter", c.platformConfig.Reporter)

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	var modifiedEntrypoints []string

	for _, dir := range c.flagDir {
		dirAbs, err := util.PathEvalAbs(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// ignore missing directories, we will just return nothing
				// resulting in a no-op for future stages
				continue
			}
			return fmt.Errorf("failed to find absolute path for directory: %w", err)
		}

		if c.flagDetectChanges {
			modifiedDirs, err := c.detectEntrypointChanges(ctx, dirAbs)
			if err != nil {
				return fmt.Errorf("failed to detect entrypoint changes: %w", err)
			}
			modifiedEntrypoints = append(modifiedEntrypoints, modifiedDirs...)

			continue
		}

		modifiedDirs, err := c.findEntrypointDirs(ctx, dirAbs)
		if err != nil {
			return fmt.Errorf("failed to find entrypoint directories: %w", err)
		}
		modifiedEntrypoints = append(modifiedEntrypoints, modifiedDirs...)
	}

	// sort them for consistent results
	slices.Sort(modifiedEntrypoints)

	results := modifiedEntrypoints

	if err := c.writeOutput(cwd, results); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if err := c.reporterClient.EntrypointsSummary(ctx, &reporter.EntrypointsSummaryParams{
		Message: "Guardian will run for the following directories",
		Dirs:    modifiedEntrypoints,
	}); err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	return nil
}

// findEntrypointDirs finds all the entrypoint directories that have a terraform module.
func (c *EntrypointsCommand) findEntrypointDirs(ctx context.Context, dir string) ([]string, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "finding entrypoint directories")

	entrypoints, err := terraform.GetEntrypointDirectories(dir, c.parsedFlagMaxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform directories: %w", err)
	}

	entrypointDirs := make([]string, 0, len(entrypoints))
	for _, e := range entrypoints {
		entrypointDirs = append(entrypointDirs, e.Path)
	}

	logger.DebugContext(ctx, "calculated entrypoints and removed dirs", "entrypoints", entrypointDirs)

	return entrypointDirs, nil
}

// detectChanges detects changed and removed entrypoints using git diff.
func (c *EntrypointsCommand) detectEntrypointChanges(ctx context.Context, dir string) ([]string, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "detecting changed entrypoints")

	gitClient := c.newGitClient(ctx, dir)

	diffDirs, err := gitClient.DiffDirsAbs(ctx, c.flagSourceRef, c.flagDestRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find git diff directories: %w", err)
	}
	logger.DebugContext(ctx, "git diff directories", "directories", diffDirs)

	moduleUsageGraph, err := terraform.ModuleUsage(ctx, dir, c.parsedFlagMaxDepth, !c.flagFailUnresolvableModules)
	if err != nil {
		return nil, fmt.Errorf("failed to get module usage for %s: %w", dir, err)
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

	modifiedDirs := maps.Keys(modifiedEntrypoints)

	return modifiedDirs, nil
}

// writeOutput writes the command output.
func (c *EntrypointsCommand) writeOutput(cwd string, results []string) error {
	// convert to child path for output
	// using absolute path creates an ugly github workflow name
	for k, dir := range results {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for [%s]: %w", dir, err)
		}
		results[k] = childPath
	}

	if results == nil {
		results = []string{}
	}
	if err := json.NewEncoder(c.Stdout()).Encode(results); err != nil {
		return fmt.Errorf("failed to create json string: %w", err)
	}

	return nil
}
