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

// Package download fetches guardian assets.
package download

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/posener/complete/v2"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ cli.Command = (*DownloadPlanCommand)(nil)

// ApplyCommand performs terraform apply on the given working directory.
type DownloadPlanCommand struct {
	cli.BaseCommand

	directory     string
	childPath     string
	planFilename  string
	storagePrefix string

	platformConfig platform.Config

	flags.CommonFlags

	flagStorage   string
	flagOutputDir string

	storageClient  storage.Storage
	platformClient platform.Platform
}

// Desc provides a short, one-line description of the command.
func (c *DownloadPlanCommand) Desc() string {
	return "Download a plan"
}

// Help is the long-form help output to include usage instructions and flag
// information.
func (c *DownloadPlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Downloads a plan.
`
}

func (c *DownloadPlanCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)
	c.CommonFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "storage",
		Target:  &c.flagStorage,
		Example: "gcs://my-guardian-state-bucket",
		Usage:   fmt.Sprintf("The storage strategy for saving Guardian plan files. Defaults to current working directory. Valid values are %q.", storage.SortedStorageTypes),
		Predict: complete.PredictFunc(func(prefix string) []string {
			return storage.SortedStorageTypes
		}),
	})

	f.StringVar(&cli.StringVar{
		Name:    "output-dir",
		Target:  &c.flagOutputDir,
		Example: "./output/plan",
		Usage:   "Write the plan binary and JSON file to a target local directory.",
	})

	return set
}

func (c *DownloadPlanCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_download_plan", 1)

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

	if c.flagStorage == "" {
		c.flagStorage = path.Join("file://", cwd)
	}

	dirAbs, err := util.PathEvalAbs(c.FlagDir)
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	childPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.childPath = childPath

	if c.flagOutputDir == "" {
		c.flagOutputDir = childPath
	}
	if c.flagOutputDir, err = filepath.Abs(c.flagOutputDir); err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platformClient = platform

	storagePrefix, err := c.platformClient.StoragePrefix(ctx)
	if err != nil {
		return fmt.Errorf("failed to parse storage flag: %w", err)
	}
	c.storagePrefix = storagePrefix

	sc, err := storage.Parse(ctx, c.flagStorage)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	c.storageClient = sc

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian apply run process.
func (c *DownloadPlanCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "starting guardian download plan",
		"platform", c.platformConfig.Type)

	util.Headerf(c.Stdout(), "Starting Guardian Download Plan")

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	planStoragePath := path.Join(c.storagePrefix, c.childPath, c.planFilename)
	logger.DebugContext(ctx, "plan storage path", "path", planStoragePath)

	planData, planExitCode, err := c.downloadGuardianPlan(ctx, planStoragePath)
	if err != nil {
		return fmt.Errorf("failed to download guardian plan file: %w", err)
	}
	logger.DebugContext(ctx, "guardian download plan", "exit_code", planExitCode)

	planAbsFilepath := path.Join(c.flagOutputDir, c.planFilename)
	file, err := os.Create(planAbsFilepath)
	if err != nil {
		return fmt.Errorf("failed to open out file %s: %w", planAbsFilepath, err)
	}
	defer file.Close()

	if _, err = file.Write(planData); err != nil {
		return fmt.Errorf("failed to write plan to %s: %w", planAbsFilepath, err)
	}

	return nil
}

// downloadGuardianPlan downloads the Guardian plan binary from the configured Guardian storage bucket
// and returns the plan data and plan exit code.
func (c *DownloadPlanCommand) downloadGuardianPlan(ctx context.Context, path string) (planData []byte, planExitCode string, outErr error) {
	util.Headerf(c.Stdout(), "Downloading Guardian plan file")

	rc, metadata, err := c.storageClient.GetObject(ctx, path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download object: %w", err)
	}

	if metadata != nil {
		exitCode, ok := metadata[plan.MetaKeyExitCode]
		if !ok {
			return nil, "", fmt.Errorf("failed to determine plan exit code: %w", err)
		}
		planExitCode = exitCode
	}

	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			outErr = fmt.Errorf("failed to close get object reader: %w", closeErr)
		}
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read plan data: %w", err)
	}
	planData = data

	return planData, planExitCode, nil
}
