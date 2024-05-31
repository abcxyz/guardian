// Copyright 2024 The Authors (see AUTHORS file)
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

// Package cleanup provides the functionality to cleanup artifacts generated with Guardian.
package cleanup

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/pkg/commands/drift/statefiles"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ cli.Command = (*CleanupCommand)(nil)

type CleanupCommand struct {
	cli.BaseCommand

	directory string

	flags.RetryFlags
	flags.CommonFlags

	flagDestRef   string
	flagSourceRef string

	gitClient       git.Git
	storageClient   storage.Storage
	terraformParser parser.Terraform
}

func (c *CleanupCommand) Desc() string {
	return `Cleanup artifacts generated with Guardian.`
}

func (c *CleanupCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Cleanup artifacts generated with Guardian.
`
}

func (c *CleanupCommand) Flags() *cli.FlagSet {
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

	return set
}

func (c *CleanupCommand) Run(ctx context.Context, args []string) error {
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

	sc, err := storage.NewGoogleCloudStorage(
		ctx,
		storage.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		storage.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)
	if err != nil {
		return fmt.Errorf("failed to create google cloud storage client: %w", err)
	}
	c.storageClient = sc

	c.terraformParser, err = parser.NewTerraformParser(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create terraform parser: %w", err)
	}

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *CleanupCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "finding modified entrypoints")
	entrypoints, err := c.getModifiedEntrypointsFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get modified entrypoints: %w", err)
	}

	logger.DebugContext(ctx, "finding statefile URIs from entrypoints")
	statefileURIs, err := statefiles.StatefileUrisFromEntrypoints(ctx, entrypoints)
	if err != nil {
		return fmt.Errorf("failed to get statefile URIs from entrypoints: %w", err)
	}

	logger.DebugContext(ctx, "finding empty statefiles from entrypoints")
	emptyStateFiles, err := statefiles.EmptyStateFiles(ctx, c.terraformParser, statefileURIs)
	if err != nil {
		return fmt.Errorf("failed to find empty statefiles: %w", err)
	}

	for _, statefileURI := range emptyStateFiles {
		logger.InfoContext(ctx, "deleting empty statefile", "uri", statefileURI)
		bucketName, objectName, err := storage.SplitObjectURI(statefileURI)
		if err != nil {
			return fmt.Errorf("failed to parse gcs object %s: %w", statefileURI, err)
		}

		if err = c.storageClient.DeleteObject(ctx, *bucketName, *objectName); err != nil {
			return fmt.Errorf("failed to delete statefile stored in gcs %s: %w", statefileURI, err)
		}
	}

	return nil
}

// Process handles the main logic for the Guardian init process.
func (c *CleanupCommand) getModifiedEntrypointsFiles(ctx context.Context) ([]string, error) {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "finding entrypoint directories")
	logger.DebugContext(ctx, "finding git diff directories")

	// TODO(dcreey): Figure out how to support deleted directories.
	// The current implementation filters out deleted directories and only
	// returns the absolute path of directories that exist at HEAD.
	diffDirs, err := c.gitClient.DiffDirsAbs(ctx, c.flagSourceRef, c.flagDestRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find git diff directories: %w", err)
	}
	logger.DebugContext(ctx, "git diff directories", "directories", diffDirs)

	moduleUsageGraph, err := terraform.ModuleUsage(ctx, c.directory, nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get module usage for %s: %w", c.directory, err)
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

	dirs := maps.Keys(modifiedEntrypoints)

	entrypointFiles := make([]string, len(dirs))
	for i, dir := range dirs {
		entrypointOfOne, err := terraform.GetEntrypointDirectories(dir, util.Ptr(1))
		if err != nil {
			return nil, fmt.Errorf("failed to find terraform directories: %w", err)
		}
		if len(entrypointOfOne) != 1 {
			return nil, fmt.Errorf("received unexpected number of entrypoints for target entrypoint with no recursion: %s - %d",
				dir, len(entrypointOfOne))
		}
		entrypointFiles[i] = entrypointOfOne[0].BackendFile
	}

	sort.Strings(entrypointFiles)
	logger.DebugContext(ctx, "target files", "target_files", entrypointFiles)

	return entrypointFiles, nil
}
