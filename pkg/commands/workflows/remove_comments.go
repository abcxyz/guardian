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
	"flag"
	"fmt"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*RemoveGuardianCommentsCommand)(nil)

type RemoveGuardianCommentsCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	platformClient platform.Platform
}

func (c *RemoveGuardianCommentsCommand) Desc() string {
	return `Remove previous Guardian comments from a pull request`
}

func (c *RemoveGuardianCommentsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Remove previous Guardian comments from a pull request.
`
}

func (c *RemoveGuardianCommentsCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)

	return set
}

func (c *RemoveGuardianCommentsCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_workflows_remove_guardian_comments", 1)

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

// Process handles the main logic for the Guardian remove plan comments process.
func (c *RemoveGuardianCommentsCommand) Process(ctx context.Context) error {
	if err := c.platformClient.ClearReports(ctx); err != nil {
		return fmt.Errorf("failed to remove comments: %w", err)
	}

	return nil
}
