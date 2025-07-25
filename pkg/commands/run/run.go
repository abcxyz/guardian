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

	"golang.org/x/exp/slices"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/pointer"
)

var _ cli.Command = (*RunCommand)(nil)

type RunCommand struct {
	cli.BaseCommand

	directory        string
	childPath        string
	terraformCommand string
	terraformArgs    []string

	flags.CommonFlags

	flagAllowedTerraformCommands []string
	flagAllowLockfileChanges     bool
	flagLockTimeout              time.Duration

	terraformClient terraform.Terraform
}

func (c *RunCommand) Desc() string {
	return `Run a Terraform command for a directory`
}

func (c *RunCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Run a Terraform command for a directory.
`
}

func (c *RunCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-terraform-commands",
		Target:  &c.flagAllowedTerraformCommands,
		Example: "plan, apply, destroy",
		Usage:   "The list of allowed Terraform commands to be run. Defaults to all commands.",
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
	metricswrap.WriteMetric(ctx, "command_run", 1)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) < 1 {
		return flag.ErrHelp
	}
	c.terraformCommand, c.terraformArgs = parsedArgs[0], parsedArgs[1:]

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
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

	tfEnvVars := []string{"TF_IN_AUTOMATION=true"}
	c.terraformClient = terraform.NewTerraformClient(c.directory, tfEnvVars)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian admin run process.
func (c *RunCommand) Process(ctx context.Context) error {
	var merr error

	util.Headerf(c.Stdout(), "Starting Guardian Run")

	if len(c.flagAllowedTerraformCommands) > 0 && !slices.Contains(c.flagAllowedTerraformCommands, c.terraformCommand) {
		sort.Strings(c.flagAllowedTerraformCommands)
		return fmt.Errorf("%s is not an allowed Terraform command.\n\nAllowed commands are %q", c.terraformCommand, c.flagAllowedTerraformCommands)
	}

	if _, ok := terraform.InitRequiredCommands[c.terraformCommand]; ok {
		c.Outf("Running Terraform init")

		lockfileMode := "none"
		if !c.flagAllowLockfileChanges {
			lockfileMode = "readonly"
		}

		util.Headerf(c.Stdout(), "Initializing Terraform")
		if _, err := c.terraformClient.Init(ctx, c.Stdout(), c.Stderr(), &terraform.InitOptions{
			Input:       pointer.To(false),
			Lockfile:    pointer.To(lockfileMode),
			LockTimeout: pointer.To(c.flagLockTimeout.String()),
		}); err != nil {
			return fmt.Errorf("failed to initialize: %w", err)
		}
	}

	util.Headerf(c.Stdout(), "Running Terraform Command")
	if _, err := c.terraformClient.Run(ctx, c.Stdout(), c.Stderr(), c.terraformCommand, c.terraformArgs...); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return merr
}
