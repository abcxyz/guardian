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
	"github.com/abcxyz/abc-updater/pkg/metrics"
	"github.com/abcxyz/guardian/internal/metricswrap"
	"os"
	"strings"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/reporter"
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

	directory      string
	platformConfig platform.Config
	flags          EnforceFlags
	commonFlags    flags.CommonFlags
	platform       platform.Platform
	reporter       reporter.Reporter
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
	c.commonFlags.Register(set)
	c.platformConfig.RegisterFlags(set)
	c.flags.Register(set)
	return set
}

// Run implements cli.Command.
func (c *EnforceCommand) Run(ctx context.Context, args []string) error {
	mClient := metrics.FromContext(ctx)
	cleanup := metricswrap.WriteMetric(ctx, mClient, "command_policy_enforce", 1)
	defer cleanup()

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platform = platform

	reporter, err := reporter.NewReporter(ctx, c.platformConfig.Reporter, &reporter.Config{GitHub: c.platformConfig.GitHub})
	if err != nil {
		return fmt.Errorf("failed to create reporter: %w", err)
	}
	c.reporter = reporter

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if c.commonFlags.FlagDir == "" {
		c.commonFlags.FlagDir = cwd
	}
	c.directory = c.commonFlags.FlagDir

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
	var teams, users []string
	var b strings.Builder
	for k, v := range *results {
		logger.DebugContext(ctx, "processing policy decision",
			"policy_name", k)

		if len(v.MissingApprovals) == 0 {
			logger.DebugContext(ctx, "no missing approvals for policy",
				"policy_name", k)
			continue
		}

		fmt.Fprintf(&b, "#### %s\n", k)
		for _, m := range v.MissingApprovals {
			teams = append(teams, m.AssignTeams...)
			users = append(users, m.AssignUsers...)

			fmt.Fprint(&b, "- **Missing approvals from one of**:\n")
			if len(m.AssignUsers) > 0 {
				fmt.Fprintf(&b, "\t - Users: %s\n", strings.Join(m.AssignUsers, ", "))
			}
			if len(m.AssignTeams) > 0 {
				fmt.Fprintf(&b, "\t - Teams: %s\n", strings.Join(m.AssignTeams, ", "))
			}

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

	if err := c.reporter.Status(ctx, reporter.StatusPolicyViolation, &reporter.StatusParams{
		Operation: "Policy Violation",
		Dir:       c.directory,
		Message:   "**NOTE**: After resolving the policy violations below, re-run the `Guardian Plan` workflow to re-evaluate policy enforcement checks.",
		Details:   b.String(),
	}); err != nil {
		return fmt.Errorf("failed to report status: %w", err)
	}
	return merr
}
