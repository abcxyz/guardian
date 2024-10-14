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

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/commands/drift/statefiles"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/pointer"
)

var _ cli.Command = (*CleanupCommand)(nil)

type CleanupCommand struct {
	cli.BaseCommand

	directory string

	flags.RetryFlags
	flags.CommonFlags

	newStorageClient func(ctx context.Context, parent string) (storage.Storage, error)
	terraformParser  parser.Terraform
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

	return set
}

func (c *CleanupCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_cleanup", 1)

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

	c.newStorageClient = func(ctx context.Context, parent string) (storage.Storage, error) {
		return storage.NewGoogleCloudStorage(ctx, parent)
	}

	c.terraformParser, err = parser.NewTerraformParser(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create terraform parser: %w", err)
	}

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *CleanupCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "determining target entrypoint backend details")
	targetEntrypoint, err := terraform.GetEntrypointDirectories(c.directory, pointer.To(0))
	if err != nil {
		return fmt.Errorf("failed to find terraform directories: %w", err)
	}
	if len(targetEntrypoint) != 1 {
		return fmt.Errorf("received unexpected number of entrypoints for target entrypoint with no recursion: %s - %d",
			c.directory, len(targetEntrypoint))
	}
	targetBackend := targetEntrypoint[0].BackendFile

	logger.DebugContext(ctx, "finding statefile URI for entrypoint")
	statefileURIs, err := statefiles.StatefileUrisFromEntrypoints(ctx, []string{targetBackend})
	if err != nil {
		return fmt.Errorf("failed to get statefile URIs from entrypoints: %w", err)
	}

	logger.DebugContext(ctx, "determining if statefile is empty from entrypoint")
	emptyStateFiles, err := statefiles.EmptyStateFiles(ctx, c.terraformParser, statefileURIs)
	if err != nil {
		return fmt.Errorf("failed to find empty statefiles: %w", err)
	}
	if len(emptyStateFiles) == 0 {
		return nil
	}
	if len(emptyStateFiles) > 1 {
		return fmt.Errorf("received unexpected number of empty statefiles for target entrypoint with no recursion: %s - %d",
			c.directory, len(emptyStateFiles))
	}

	statefileURI := emptyStateFiles[0]
	logger.InfoContext(ctx, "deleting empty statefile", "uri", statefileURI)
	bucketName, objectName, err := storage.SplitObjectURI(statefileURI)
	if err != nil {
		return fmt.Errorf("failed to parse gcs object %s: %w", statefileURI, err)
	}

	sc, err := c.newStorageClient(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	if err = sc.DeleteObject(ctx, objectName); err != nil {
		return fmt.Errorf("failed to delete statefile stored in gcs %s: %w", statefileURI, err)
	}

	return nil
}
