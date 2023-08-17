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

package run

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"time"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/exp/slices"
)

var _ cli.Command = (*RunCommand)(nil)

type RunCommand struct {
	cli.BaseCommand

	directory string
	childPath string

	flags.GitHubFlags
	flags.RetryFlags

	flagAllowedTerraformCommands []string
	flagTerraformCommand         string
	flagTerraformArgs            []string
	flagAllowLockfileChanges     bool
	flagLockTimeout              time.Duration

	actions         *githubactions.Action
	terraformClient terraform.Terraform
}

func (c *RunCommand) Desc() string {
	return `Run a Terraform command for a directory`
}

func (c *RunCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

  Run a Terraform command for a directory.
`
}

func (c *RunCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-terraform-commands",
		Target:  &c.flagAllowedTerraformCommands,
		Example: "plan, apply, destroy",
		Usage:   "The list of allowed Terraform commands to be run. Defaults to all commands.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "terraform-command",
		Target:  &c.flagTerraformCommand,
		Example: "plan",
		Usage:   "The Terraform command to run.",
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "terraform-args",
		Target:  &c.flagTerraformArgs,
		Example: "-no-color,-input=false",
		Usage:   "The list arguments to provide to the Terraform CLI command.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "allow-lockfile-changes",
		Target:  &c.flagAllowLockfileChanges,
		Example: "true",
		Usage:   "Allow modification of the Terraform lockfile.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "lock-timeout",
		Target:  &c.flagLockTimeout,
		Default: 10 * time.Minute,
		Example: "10m",
		Usage:   "The duration Terraform should wait to obtain a lock when running commands that modify state.",
	})

	return set
}

func (c *RunCommand) Run(ctx context.Context, args []string) error {
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

	childPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.childPath = childPath

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	c.terraformClient = terraform.NewTerraformClient(c.directory)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian admin run process.
func (c *RunCommand) Process(ctx context.Context) error {
	var merr error

	c.Outf("Starting Guardian run")

	if len(c.flagAllowedTerraformCommands) > 0 && !slices.Contains(c.flagAllowedTerraformCommands, c.flagTerraformCommand) {
		sort.Strings(c.flagAllowedTerraformCommands)
		return fmt.Errorf("%s is not an allowed Terraform command.\n\nAllowed commands are %q", c.flagTerraformCommand, c.flagAllowedTerraformCommands)
	}

	if _, ok := terraform.InitRequiredCommands[c.flagTerraformCommand]; ok {
		c.Outf("Running Terraform init")

		lockfileMode := "none"
		if !c.flagAllowLockfileChanges {
			lockfileMode = "readonly"
		}

		if err := c.withActionsOutGroup("Initializing Terraform", func() error {
			_, err := c.terraformClient.Init(ctx, c.Stdout(), c.Stderr(), &terraform.InitOptions{
				Input:       util.Ptr(false),
				Lockfile:    util.Ptr(lockfileMode),
				LockTimeout: util.Ptr(c.flagLockTimeout.String()),
			})
			return err //nolint:wrapcheck // Want passthrough
		}); err != nil {
			return fmt.Errorf("failed to initialize: %w", err)
		}
	}

	if err := c.withActionsOutGroup("Running Terraform command", func() error {
		_, err := c.terraformClient.Run(ctx, c.Stdout(), c.Stderr(), c.flagTerraformCommand, c.flagTerraformArgs...)
		return err //nolint:wrapcheck // Want passthrough
	}); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return merr
}

// withActionsOutGroup runs a function and ensures it is wrapped in GitHub actions
// grouping syntax. If this is not in an action, output is printed without grouping syntax.
func (c *RunCommand) withActionsOutGroup(msg string, fn func() error) error {
	if c.GitHubFlags.FlagIsGitHubActions {
		c.actions.Group(msg)
		defer c.actions.EndGroup()
	} else {
		c.Outf(msg)
	}

	return fn()
}
