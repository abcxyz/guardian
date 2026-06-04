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

package workflows

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*ReportCommand)(nil)

type ReportCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	flagType         string
	flagEntrypoints  string
	flagArtifactsDir string

	platformClient platform.Platform
}

func (c *ReportCommand) Desc() string {
	return `Aggregate and report Guardian plan/apply status on pull requests`
}

func (c *ReportCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Aggregate and report Guardian plan/apply status on pull requests.
`
}

func (c *ReportCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "type",
		Target:  &c.flagType,
		Example: "plan",
		Usage:   "The type of the report, either 'plan' or 'apply'.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "entrypoints",
		Target:  &c.flagEntrypoints,
		Example: `["terraform/github/abseil"]`,
		Usage:   "The list of directory entrypoints as a JSON array string.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "artifacts-dir",
		Target:  &c.flagArtifactsDir,
		Example: "./artifacts",
		Usage:   "The local path where plan artifacts are downloaded (required for plan type).",
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.flagType != "plan" && c.flagType != "apply" {
			merr = errors.Join(merr, fmt.Errorf("missing or invalid flag: type must be 'plan' or 'apply'"))
		}

		if c.flagEntrypoints == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: entrypoints is required"))
		}

		if c.flagType == "plan" && c.flagArtifactsDir == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: artifacts-dir is required when type is 'plan'"))
		}

		return merr
	})

	return set
}

func (c *ReportCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_workflows_report", 1)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platformClient = platform

	return c.Process(ctx)
}

func (c *ReportCommand) Process(ctx context.Context) error {
	// Stub implementation for initial PR
	c.Outf("report command scaffolding works. type: %s, entrypoints: %s, artifacts: %s", c.flagType, c.flagEntrypoints, c.flagArtifactsDir)
	return nil
}
