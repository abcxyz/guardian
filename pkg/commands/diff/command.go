// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package diff

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
)

var _ cli.Command = (*DiffCommand)(nil)

// DiffCommand is a subcommand for Guardian that allows detecting which terraform entrypoints have changed.
type DiffCommand struct {
	cli.BaseCommand

	directory     string
	diffChildPath string

	flags.GitHubFlags

	flagBaseRef                 string
	flagHeadRef                 string
	flagFailUnresolvableModules bool
}

func (c *DiffCommand) Desc() string {
	return `Detect changes to terraform entrypoints`
}

func (c *DiffCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Detect changes to terraform entrypoints.
`
}

func (c *DiffCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "base-ref",
		Target:  &c.flagBaseRef,
		Example: "896ed966ff21d91f0fe15e7674e655bd0807656b",
		Usage:   `The base ref to diff against.`,
	})

	f.StringVar(&cli.StringVar{
		Name:    "head-ref",
		Target:  &c.flagHeadRef,
		Example: "454e59d750bb6d0b158829291e4bf1e8ddde3b72",
		Usage:   `The head ref to diff against.`,
	})
	f.BoolVar(&cli.BoolVar{
		Name:    "fail-unresolvable-modules",
		Target:  &c.flagFailUnresolvableModules,
		Usage:   `Whether or not to error if a module cannot be resolved.`,
		Default: false,
	})

	return set
}

func (c *DiffCommand) Run(ctx context.Context, args []string) error {
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

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	diffChildPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.diffChildPath = diffChildPath

	logger := logging.FromContext(ctx)

	logger.Debugw("Running Diff...",
		"name", version.Name,
		"commit", version.Commit,
		"version", version.Version)

	gitClient := git.NewGitClient(c.directory)

	diffs, err := gitClient.DiffDirsAbs(ctx, c.flagBaseRef, c.flagHeadRef)
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	moduleUsageGraph, err := terraform.ModuleUsage(ctx, c.directory, !c.flagFailUnresolvableModules)
	if err != nil {
		return fmt.Errorf("failed to get module usage for %s: %w", c.diffChildPath, err)
	}

	modifiedEntrypoints := make(map[string]struct{})

	for _, changedFile := range diffs {
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

	for _, f := range files {
		c.Outf(f)
	}

	return nil
}
