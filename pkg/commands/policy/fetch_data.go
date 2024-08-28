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

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*FetchDataCommand)(nil)

// FetchDataCommand implements cli.Command. It fetches and aggregates data for
// a target platform to be used for policy evaluation.
type FetchDataCommand struct {
	cli.BaseCommand

	platformConfig platform.Config
	platform       platform.Platform
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

	if err = json.NewEncoder(c.Stdout()).Encode(data); err != nil {
		return fmt.Errorf("failed to write json to output: %w", err)
	}
	return nil
}
