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

	"github.com/posener/complete/v2"
	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
)

var _ cli.Command = (*EntrypointsCommand)(nil)

// allowedFormats are the allowed format flags for this command.
var allowedFormats = map[string]struct{}{
	"json": {},
	"text": {},
}

// allowedFormatNames are the sorted allowed format names for the format flag.
// This is used for printing messages and prediction.
var allowedFormatNames = util.SortedMapKeys(allowedFormats)

// EntrypointsResult is the entryponts command result.
type EntrypointsResult struct {
	Entrypoints []string `json:"entrypoints"`
	Changed     []string `json:"changed"`
	Removed     []string `json:"removed"`
}

// Params are the inputs required to process entrypoints.
type Params struct {
	destEntrypoints   []*terraform.TerraformEntrypoint
	sourceEntrypoints []*terraform.TerraformEntrypoint
}

type EntrypointsCommand struct {
	cli.BaseCommand

	directory string

	flags.RetryFlags
	flags.CommonFlags

	flagDestRef                 string
	flagSourceRef               string
	flagDetectChanges           bool
	flagFormat                  string
	flagFailUnresolvableModules bool
	flagMaxDepth                int

	parsedFlagMaxDepth *int

	gitClient git.Git
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

	c.RetryFlags.Register(set)
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

		if _, ok := allowedFormats[c.flagFormat]; !ok {
			merr = errors.Join(merr, fmt.Errorf("invalid flag: format %s (supported formats are: %s)", c.flagFormat, allowedFormatNames))
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

	if err := c.gitClient.Fetch(ctx, "origin", c.flagDestRef, c.flagSourceRef); err != nil {
		return err //nolint:wrapcheck // Want passthrough
	}

	destEntrypoints := make([]*terraform.TerraformEntrypoint, 0)
	if c.flagDestRef != "" {
		destEntrypoints, err = c.getEntrypointsForRef(ctx, c.flagDestRef)
		if err != nil {
			return fmt.Errorf("failed to find terraform directories for destRef(%s): %w", c.flagDestRef, err)
		}
	}

	sourceEntrypoints := make([]*terraform.TerraformEntrypoint, 0)
	if c.flagDestRef != "" {
		sourceEntrypoints, err = c.getEntrypointsForRef(ctx, c.flagSourceRef)
		if err != nil {
			return fmt.Errorf("failed to find terraform directories for sourceRef(%s): %w", c.flagSourceRef, err)
		}
	} else {
		sourceEntrypoints, err = terraform.GetEntrypointDirectories(c.directory, c.parsedFlagMaxDepth)
		if err != nil {
			return fmt.Errorf("failed to find terraform directories: %w", err)
		}
	}

	return c.Process(ctx, &Params{
		destEntrypoints:   destEntrypoints,
		sourceEntrypoints: sourceEntrypoints,
	})
}

// Process handles the main logic for the Guardian init process.
func (c *EntrypointsCommand) Process(ctx context.Context, p *Params) error {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "finding entrypoint directories")

	destEntrypointDirs := make([]string, 0, len(p.destEntrypoints))
	for _, e := range p.destEntrypoints {
		destEntrypointDirs = append(destEntrypointDirs, e.Path)
	}

	sourceEntrypointDirs := make([]string, 0, len(p.sourceEntrypoints))
	for _, e := range p.sourceEntrypoints {
		sourceEntrypointDirs = append(sourceEntrypointDirs, e.Path)
	}

	removedEntrypointDirs := sets.Subtract(destEntrypointDirs, sourceEntrypointDirs)

	if c.flagDetectChanges {
		logger.DebugContext(ctx, "finding git diff directories")

		diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagSourceRef, c.flagDestRef)
		if err != nil {
			return fmt.Errorf("failed to find git diff directories: %w", err)
		}
		logger.DebugContext(ctx, "git diff directories", "directories", diffDirs)

		moduleUsageGraph, err := terraform.ModuleUsage(ctx, c.directory, c.parsedFlagMaxDepth, !c.flagFailUnresolvableModules)
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

		sourceEntrypointDirs = files
	}

	allEntrypointDirs := make([]string, 0, len(sourceEntrypointDirs))
	allEntrypointDirs = append(allEntrypointDirs, sourceEntrypointDirs...)
	allEntrypointDirs = append(allEntrypointDirs, removedEntrypointDirs...)

	logger.DebugContext(ctx, "all target directories", "all_directories", allEntrypointDirs)

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	allEntrypoints, err := c.mapDirsToChildPath(cwd, allEntrypointDirs)
	if err != nil {
		return err
	}

	changedEntrypoints, err := c.mapDirsToChildPath(cwd, sourceEntrypointDirs)
	if err != nil {
		return err
	}

	removedEntrypoints, err := c.mapDirsToChildPath(cwd, removedEntrypointDirs)
	if err != nil {
		return err
	}

	if err := c.writeOutput(&EntrypointsResult{
		Entrypoints: allEntrypoints,
		Changed:     changedEntrypoints,
		Removed:     removedEntrypoints,
	}); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

func (c *EntrypointsCommand) getEntrypointsForRef(ctx context.Context, ref string) ([]*terraform.TerraformEntrypoint, error) {
	if err := c.gitClient.Checkout(ctx, ref); err != nil {
		return nil, err //nolint:wrapcheck // Want passthrough
	}

	entrypoints, err := terraform.GetEntrypointDirectories(c.directory, c.parsedFlagMaxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform directories: %w", err)
	}

	return entrypoints, nil
}

// mapDirsToChildPath maps directories to child paths.
func (c *EntrypointsCommand) mapDirsToChildPath(cwd string, dirs []string) ([]string, error) {
	resp := make([]string, 0, len(dirs))

	// convert to child path for output
	// using absolute path creates an ugly github workflow name
	for _, dir := range dirs {
		childPath, err := util.ChildPath(cwd, dir)
		if err != nil {
			return nil, fmt.Errorf("failed to get child path for: %w", err)
		}
		resp = append(resp, childPath)
	}

	return resp, nil
}

// writeOutput writes the command output.
func (c *EntrypointsCommand) writeOutput(dirs *EntrypointsResult) error {
	switch v := strings.TrimSpace(strings.ToLower(c.flagFormat)); v {
	case "json":
		if err := json.NewEncoder(c.Stdout()).Encode(dirs); err != nil {
			return fmt.Errorf("failed to create json string: %w", err)
		}
	case "text":
		for _, dir := range dirs.Changed {
			c.Outf("%s", dir)
		}
		for _, dir := range dirs.Removed {
			c.Outf("DESTROY::%s", dir)
		}
	default:
		return fmt.Errorf("invalid format flag: %s (supported formats are: %s)", c.flagFormat, allowedFormatNames)
	}

	return nil
}
