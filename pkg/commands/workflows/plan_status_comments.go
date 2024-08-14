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

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*PlanStatusCommentCommand)(nil)

type PlanStatusCommentCommand struct {
	cli.BaseCommand

	githubConfig github.Config

	flagReporter   string
	flagInitResult string
	flagPlanResult []string

	reporterClient reporter.Reporter
}

func (c *PlanStatusCommentCommand) Desc() string {
	return `Remove previous Guardian plan comments from a pull request`
}

func (c *PlanStatusCommentCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Remove previous Guardian plan comments from a pull request.
`
}

func (c *PlanStatusCommentCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.githubConfig.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "reporter",
		Target:  &c.flagReporter,
		Example: "github",
		Default: "local",
		Usage:   "The reporting strategy for Guardian status updates.",
	})

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
func (c *PlanStatusCommentCommand) Process(ctx context.Context) error {
	// this case improves user experience, when the planning job does not have a plan diff
	// there are no comments, this informs the user that the job ran successfully
	if c.flagInitResult == "success" && util.SliceContainsOnly(c.flagPlanResult, "success") {
		err := c.reporterClient.CreateStatus(ctx, reporter.StatusSuccess, &reporter.Params{Operation: "plan", Message: "Plan completed successfully."})
		if err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}
		return nil
	}

	// this case does not require a comment because the planning job should
	// have commented that there was a failure and for which directory
	if c.flagInitResult == "failure" || slices.Contains(c.flagPlanResult, "failure") {
		return fmt.Errorf("init or plan has one or more failures")
	}

	// this case improves user experience, when no Terraform changes were submitted
	// and the plan job is skipped, this informs the user that the job ran successfully
	// but no changes are needed
	if c.flagInitResult == "success" && util.SliceContainsOnly(c.flagPlanResult, "skipped") {
		err := c.reporterClient.CreateStatus(ctx, reporter.StatusNoOperation, &reporter.Params{Operation: "plan", Message: "No Terraform changes detected, planning skipped."})
		if err != nil {
			return fmt.Errorf("failed to create plan status comment: %w", err)
		}
		return nil
	}

	err := c.reporterClient.CreateStatus(ctx, reporter.StatusUnknown, &reporter.Params{Operation: "plan", Message: "Unable to determine plan status."})
	if err != nil {
		return fmt.Errorf("failed to create plan status comment: %w", err)
	}

	return fmt.Errorf("unable to determine plan status")
}
