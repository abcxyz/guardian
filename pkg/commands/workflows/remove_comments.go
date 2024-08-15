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

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*RemoveGuardianCommentsCommand)(nil)

type RemoveGuardianCommentsCommand struct {
	cli.BaseCommand

	githubConfig github.Config

	flagReporter string

	reporterClient reporter.Reporter
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

	c.githubConfig.RegisterFlags(set)

	return set
}

func (c *RemoveGuardianCommentsCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	rc, err := reporter.NewReporter(ctx, c.flagReporter, &reporter.Config{GitHub: c.githubConfig}, c.Stdout())
	if err != nil {
		return fmt.Errorf("failed to create reporter client: %w", err)
	}
	c.reporterClient = rc

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian remove plan comments process.
func (c *RemoveGuardianCommentsCommand) Process(ctx context.Context) error {
	if err := c.reporterClient.ClearStatus(ctx); err != nil {
		return fmt.Errorf("failed to remove comments: %w", err)
	}

	return nil
}
