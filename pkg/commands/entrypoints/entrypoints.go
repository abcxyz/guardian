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
	"os"
	"path"
	"slices"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/modifiers"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
)

var _ cli.Command = (*EntrypointsCommand)(nil)

// EntrypointsResult is the entryponts command result.
type EntrypointsResult struct {
	Entrypoints []string `json:"entrypoints"`
	Modified    []string `json:"modified"`
	Destroy     []string `json:"destroy"`
}

type EntrypointsCommand struct {
	cli.BaseCommand

	directory string

	platformConfig platform.Config

	flags.CommonFlags

	flagDestRef                 string
	flagSourceRef               string
	flagDetectChanges           bool
	flagFailUnresolvableModules bool
	flagMaxDepth                int

	parsedFlagMaxDepth *int

	gitClient      git.Git
	platformClient platform.Platform
	reporterClient reporter.Reporter
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

	c.platformConfig.RegisterFlags(set)
	c.CommonFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

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

	dirAbs, err := util.PathEvalAbs(c.FlagDir)
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	c.gitClient = git.NewGitClient(c.directory)

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

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	entrypointDirs, removedDirs, err := c.findEntrypointDirs(ctx)
	if err != nil {
		return fmt.Errorf("failed to find entrypoint directories: %w", err)
	}

	metaValues := modifiers.ParseBodyMetaValues(ctx, c.platformClient.ModifierContent(ctx))
	logger.DebugContext(ctx, "parsed body meta values", "values", metaValues)

	allEntrypointDirs := slices.Concat(nil, entrypointDirs, removedDirs)

	destroyDirs, err := c.processDestroyMetaValues(cwd, allEntrypointDirs, metaValues)
	if err != nil {
		return fmt.Errorf("failed to find entrypoint directories: %w", err)
	}
	logger.DebugContext(ctx, "found destroy dirs from meta values", "dirs", destroyDirs)

	modifiedDirs := sets.Subtract(entrypointDirs, destroyDirs)

	// TODO(verbanicm): write a comment to help the user with abandonded dirs
	abandonedDirs := sets.Subtract(removedDirs, destroyDirs)
	logger.DebugContext(ctx, "found abandonded dirs", "dirs", abandonedDirs)

	results := &EntrypointsResult{
		Entrypoints: entrypointDirs,
		Modified:    modifiedDirs,
		Destroy:     destroyDirs,
	}

	if err := c.writeOutput(cwd, results); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if err := c.reporterClient.EntrypointsSummary(ctx, &reporter.EntrypointsSummaryParams{
		Message:       "Guardian will run for the following directories",
		ModifiedDirs:  modifiedDirs,
		DestroyDirs:   destroyDirs,
		AbandonedDirs: abandonedDirs,
	}); err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	return nil
}

// FindEntrypointDirs finds all the entrypoint directories.
func (c *EntrypointsCommand) findEntrypointDirs(ctx context.Context) ([]string, []string, error) {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "finding entrypoint directories")

	entrypoints, err := terraform.GetEntrypointDirectories(c.directory, c.parsedFlagMaxDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find terraform directories: %w", err)
	}

	entrypointDirs := make([]string, 0, len(entrypoints))
	for _, e := range entrypoints {
		entrypointDirs = append(entrypointDirs, e.Path)
	}

	logger.DebugContext(ctx, "terraform entrypoint directories", "entrypoint_dirs", entrypointDirs)

	removedDirs := make([]string, 0)
	if c.flagDetectChanges {
		logger.DebugContext(ctx, "finding git diff directories")

		diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagSourceRef, c.flagDestRef)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find git diff directories: %w", err)
		}
		logger.DebugContext(ctx, "git diff directories", "directories", diffDirs)

		removedDirs, err = c.findRemovedDirs(diffDirs)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find removed dirs: %w", err)
		}

		moduleUsageGraph, err := terraform.ModuleUsage(ctx, c.directory, c.parsedFlagMaxDepth, !c.flagFailUnresolvableModules)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get module usage for %s: %w", c.directory, err)
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

	logger.DebugContext(ctx, "calculated entrypoints and removed dirs",
		"entrypoints", entrypointDirs,
		"removed_dirs", removedDirs,
	)

	return entrypointDirs, removedDirs, nil
}

// findRemovedDirs tests if any directories were removed from the filesystem and returns the list of removed dirs.
func (c *EntrypointsCommand) findRemovedDirs(dirs []string) ([]string, error) {
	removed := make([]string, 0)

	for _, dir := range dirs {
		_, err := os.Stat(dir)
		// there was an error testing if dir exists
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to check if dir exists: %w", err)
		}

		if errors.Is(err, fs.ErrNotExist) {
			removed = append(removed, dir)
		}
	}

	return removed, nil
}

// processDestroyMetaValues processes the user supplied meta values for directories to destroy.
func (c *EntrypointsCommand) processDestroyMetaValues(cwd string, dirs []string, metaValues modifiers.MetaValues) ([]string, error) {
	metaDestroyDirs, ok := metaValues[modifiers.MetaKeyGuardianDestroy]
	if !ok {
		return []string{}, nil
	}

	for i, dir := range metaDestroyDirs {
		metaDestroyDirs[i] = path.Join(cwd, dir)
	}

	destroyDirs := sets.Intersect(dirs, metaDestroyDirs)
	return destroyDirs, nil
}

// writeOutput writes the command output.
func (c *EntrypointsCommand) writeOutput(cwd string, results *EntrypointsResult) error {
	// convert to child path for output
	// using absolute path creates an ugly github workflow name
	for k, dir := range results.Entrypoints {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for [%s]: %w", dir, err)
		}
		results.Entrypoints[k] = childPath
	}

	for k, dir := range results.Modified {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for [%s]: %w", dir, err)
		}
		results.Modified[k] = childPath
	}

	for k, dir := range results.Destroy {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return fmt.Errorf("failed to get child path for [%s]: %w", dir, err)
		}
		results.Destroy[k] = childPath
	}

	if err := json.NewEncoder(c.Stdout()).Encode(results); err != nil {
		return fmt.Errorf("failed to create json string: %w", err)
	}

	return nil
}
