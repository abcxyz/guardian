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

package plan

import (
	"context"
	"fmt"

	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*PlanCommand)(nil)

type PlanCommand struct {
	cli.BaseCommand

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option
}

func (p *PlanCommand) Desc() string {
	return `Run the Terraform plan for a directory.`
}

func (p *PlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Run the Terraform plan for a directory.
`
}

func (p *PlanCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(p.testFlagSetOpts...)
	return set
}

func (p *PlanCommand) Run(ctx context.Context, args []string) error {
	f := p.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	p.Outf("Running Guardian plan...")

	return nil
}
