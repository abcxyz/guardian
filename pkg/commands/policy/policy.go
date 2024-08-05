// Copyright 2024 The Authors (see AUTHORS file)
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

// Package policy implements the policy command for enforcing policies.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

// Result defines the expected structure of the OPA policy evaluation result.
type Result struct {
	MissingApprovals []MissingApproval `json:"missing_approvals"`
}

// MissingApproval defines the missing approvals determined from the policy
// evaluation result.
type MissingApproval struct {
	AssignTeams []string `json:"assign_team_reviewers"`
	AssignUsers []string `json:"assign_user_reviewers"`
	Message     string   `json:"msg"`
}

// Results is a map of the policy package name to the policy evaluation result.
type Results map[string]*Result

var _ cli.Command = (*PolicyCommand)(nil)

type PolicyCommand struct {
	cli.BaseCommand
	flags PolicyFlags
}

// Desc implements cli.Command.
func (c *PolicyCommand) Desc() string {
	return "Enforce a set of Guardian policies"
}

// Help implements cli.Command.
func (c *PolicyCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Enforce the results of OPA policy decisions.
`
}

// Flags returns the list of flags that are defined on the command.
func (c *PolicyCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet()
	c.flags.Register(set)
	return set
}

// Run implements cli.Command.
func (c *PolicyCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	logger.DebugContext(ctx, "parsing results file",
		"results_file", c.flags.ResultsFile)
	d, err := os.ReadFile(c.flags.ResultsFile)
	if err != nil {
		return fmt.Errorf("failed to read results file %q: %w", c.flags.ResultsFile, err)
	}

	var results *Results
	if err := json.Unmarshal(d, &results); err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return nil
}
