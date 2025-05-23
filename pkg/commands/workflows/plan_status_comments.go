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
	"errors"
	"flag"
	"fmt"
	"slices"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*PlanStatusCommentCommand)(nil)

type PlanStatusCommentCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	flagInitResult    string
	flagPlanResult    []string
	flagSkipReporting bool

	platformClient platform.Platform
}

func (c *PlanStatusCommentCommand) Desc() string {
	return `Report the status of a Guardian plan`
}

func (c *PlanStatusCommentCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Report the status of a Guardian plan.
`
}

func (c *PlanStatusCommentCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "init-result",
		Target:  &c.flagInitResult,
		Example: "success",
		Usage:   "The Guardian init job result status.",
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "plan-result",
		Target:  &c.flagPlanResult,
		Example: "failure",
		Usage:   "The Guardian plan job result status.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "skip-reporting",
		Target:  &c.flagSkipReporting,
		Default: false,
		Example: "true",
		Usage:   "Skips reporting of the plan status on the change request.",
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.flagInitResult == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: init-result is required"))
		}

		if len(c.flagPlanResult) == 0 {
			merr = errors.Join(merr, fmt.Errorf("missing flag: plan-result is required"))
		}

		return merr
	})

	return set
}

func (c *PlanStatusCommentCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_workflows_plan_status_comments", 1)

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

// Process handles the main logic for the Guardian plan status comments process.
func (c *PlanStatusCommentCommand) Process(ctx context.Context) error {
	// there was at least one failure, we should return an error to fail the job
	// no comments as each plan run will comment their failure status
	if c.flagInitResult == github.GitHubWorkflowResultFailure || slices.Contains(c.flagPlanResult, github.GitHubWorkflowResultFailure) {
		return fmt.Errorf("init or plan has one or more failures")
	}

	if c.flagSkipReporting {
		return nil
	}

	// all plan runs were skipped, meaning there were no changes to plan
	// no plans were run so there will be no comments, we can improve user experience
	// by showing status that there were no changes to be planned
	if c.flagInitResult == github.GitHubWorkflowResultSuccess && util.SliceContainsOnly(c.flagPlanResult, github.GitHubWorkflowResultSkipped) {
		err := c.platformClient.ReportStatus(ctx, platform.StatusNoOperation, &platform.StatusParams{Operation: "plan", Message: "No Terraform changes detected, planning skipped."})
		if err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}
		return nil
	}

	return nil
}
