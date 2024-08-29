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

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*FetchDataCommand)(nil)

const (
	ownerReadWritePerms = 0o600
	policyDataFilename  = "guardian_policy_context.json"
)

// FetchDataCommand implements cli.Command. It fetches and aggregates data for
// a target platform to be used for policy evaluation.
type FetchDataCommand struct {
	cli.BaseCommand

	platformConfig platform.Config
	platform       platform.Platform
	flags          FetchDataFlags
}

// Desc implements cli.Command.
func (c *FetchDataCommand) Desc() string {
	return "Fetch data used for policy evaluation"
}

// Help implements cli.Command.
func (c *FetchDataCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Fetch and aggregate data for the target platform to be used for policy
  evaluation.
`
}

// Flags returns the list of flags that are defined on the command.
func (c *FetchDataCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet()
	c.platformConfig.RegisterFlags(set)
	c.flags.Register(set)

	return set
}

// Run implements cli.Command.
func (c *FetchDataCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platform = platform

	return c.Process(ctx)
}

// Process handles the main logic for aggregating data for the target platform.
func (c *FetchDataCommand) Process(ctx context.Context) error {
	data, err := c.platform.GetPolicyData(ctx)
	if err != nil {
		return fmt.Errorf("failed to get policy data: %w", err)
	}

	d, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal policy data: %w", err)
	}

	fp := path.Join(c.flags.flagOutputDir, policyDataFilename)
	if err := os.WriteFile(fp, d, ownerReadWritePerms); err != nil {
		return fmt.Errorf("failed to write policy data to json file: %w", err)
	}

	absFilepath, err := util.PathEvalAbs(fp)
	if err != nil {
		return fmt.Errorf("failed to evaluate absolute filepath: %w", err)
	}
	c.Outf("Saved policy data to local file: %s", absFilepath)
	return nil
}
