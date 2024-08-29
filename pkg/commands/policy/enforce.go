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
	"errors"
	"fmt"
	"os"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

// Result defines the expected structure of the OPA policy evaluation result.
type Result struct {
	MissingApprovals []*MissingApproval `json:"missing_approvals"`
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

var _ cli.Command = (*EnforceCommand)(nil)

type EnforceCommand struct {
	cli.BaseCommand

	platformConfig platform.Config
	flags          EnforceFlags
	platform       platform.Platform
}

// Desc implements cli.Command.
func (c *EnforceCommand) Desc() string {
	return "Enforce a set of Guardian policies"
}

// Help implements cli.Command.
func (c *EnforceCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Enforce the results of OPA policy decisions.
`
}

// Flags returns the list of flags that are defined on the command.
func (c *EnforceCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet()
	c.platformConfig.RegisterFlags(set)
	c.flags.Register(set)
	return set
}

// Run implements cli.Command.
func (c *EnforceCommand) Run(ctx context.Context, args []string) error {
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

// Process handles the main logic for handling the results of the policy
// evaluation.
func (c *EnforceCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx)

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

	var merr error
	var teams []string
	var users []string
	for k, v := range *results {
		logger.DebugContext(ctx, "processing policy decision",
			"policy_name", k)

		if len(v.MissingApprovals) == 0 {
			logger.DebugContext(ctx, "no missing approvals for policy",
				"policy_name", k)
			continue
		}

		for _, m := range v.MissingApprovals {
			teams = append(teams, m.AssignTeams...)
			users = append(users, m.AssignUsers...)

			merr = errors.Join(merr, fmt.Errorf("failed: \"%s\" - %s", k, m.Message))
		}
	}

	// Skips assigning reviewers but returns any errors found. This is possible if
	// the rego policy is misconfigured/contains a bug to return an error message
	// without any reviewers to assign.
	if len(teams) == 0 && len(users) == 0 {
		return merr
	}

	logger.DebugContext(ctx, "found missing approvals",
		"teams", teams,
		"users", users,
	)
	if _, err := c.platform.AssignReviewers(ctx, &platform.AssignReviewersInput{
		Teams: teams,
		Users: users,
	}); err != nil {
		return fmt.Errorf("failed to assign reviewers: %w", err)
	}

	return merr
}