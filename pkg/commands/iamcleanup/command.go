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

// Package iamcleanup provides the functionality to cleanup IAM memberships.
package iamcleanup

import (
	"context"
	"fmt"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/internal/version"
)

var _ cli.Command = (*IAMCleanupCommand)(nil)

// IAMCleanupCommand is a subcommand for Guardian that enables cleaning up IAM.
type IAMCleanupCommand struct {
	cli.BaseCommand

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option

	flagScope                    string
	flagIAMQuery                 string
	flagDisableEvaluateCondition bool
	flagMaxConcurrentRequests    int64
}

func (c *IAMCleanupCommand) Desc() string {
	return `Detect IAM drift in a GCP organization`
}

func (c *IAMCleanupCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Detect IAM drift in a GCP organization.
`
}

func (c *IAMCleanupCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "scope",
		Target:  &c.flagScope,
		Example: "123435456456",
		Usage: `The scope to cleanup IAM for - organizations/123456 will cleanup all 
		IAM matching your query in the organization and all folders and projects beneath it.`,
	})
	f.StringVar(&cli.StringVar{
		Name:    "iam-query",
		Target:  &c.flagIAMQuery,
		Example: "policy:abcxyz-aod-expiry",
		Usage:   `The query to use to filter on IAM.`,
	})
	f.BoolVar(&cli.BoolVar{
		Name:    "disable-evaluate-condition",
		Target:  &c.flagDisableEvaluateCondition,
		Example: "true",
		Default: false,
		Usage: `Whether or not to evaluate the IAM Condition Expression and only delete
		those IAM with false evaluation. Defaults to false.
		Example: An IAM condition with expression 'request.time < timestamp("2019-01-01T00:00:00Z")'
		will evaluate to false and the IAM will be deleted.`,
	})
	f.Int64Var(&cli.Int64Var{
		Name:    "max-conncurrent-requests",
		Target:  &c.flagMaxConcurrentRequests,
		Example: "2",
		Usage:   `The maximum number of concurrent requests allowed at any time to GCP.`,
		Default: 10,
	})

	return set
}

func (c *IAMCleanupCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()

	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	logger := logging.FromContext(ctx)

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	logger.Debugw("Running IAM Cleanup...",
		"name", version.Name,
		"commit", version.Commit,
		"version", version.Version)

	if c.flagScope == "" {
		return fmt.Errorf("missing -scope")
	}

	iamCleaner, err := NewIAMCleaner(ctx, c.flagMaxConcurrentRequests)
	if err != nil {
		return fmt.Errorf("failed to create iam cleaner: %w", err)
	}
	if err := iamCleaner.Do(ctx, c.flagScope, c.flagIAMQuery, !c.flagDisableEvaluateCondition); err != nil {
		return fmt.Errorf("failed to cleanup iam: %w", err)
	}

	return nil
}
