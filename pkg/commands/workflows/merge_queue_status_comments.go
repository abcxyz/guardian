// Copyright 2025 The Authors (see AUTHORS file)
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

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*MergeQueueStatusCommentCommand)(nil)

type MergeQueueStatusCommentCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	flagResult        string
	flagTargetBranch  string
	flagSkipReporting bool

	platformClient platform.Platform
}

func (c *MergeQueueStatusCommentCommand) Desc() string {
	return `Report the status of a merge queue check`
}

func (c *MergeQueueStatusCommentCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Report the status of a merge queue check.
`
}

func (c *MergeQueueStatusCommentCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "result",
		Target:  &c.flagResult,
		Example: "success",
		Usage:   "The Guardian merge queue check result status.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "target-branch",
		Target:  &c.flagTargetBranch,
		Example: "main",
		Usage:   "The target branch that will be merged into.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "skip-reporting",
		Target:  &c.flagSkipReporting,
		Default: false,
		Example: "true",
		Usage:   "Skips reporting of the merge queue status on the change request.",
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.flagResult == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: result is required"))
		}

		return merr
	})

	return set
}

func (c *MergeQueueStatusCommentCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_workflows_merge_queue_status_comments", 1)

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

// Process handles the main logic for the Guardian merge queue status comments process.
func (c *MergeQueueStatusCommentCommand) Process(ctx context.Context) error {
	if c.flagSkipReporting {
		return nil
	}

	if c.flagResult == github.GitHubWorkflowResultFailure {
		err := c.platformClient.ReportStatus(ctx, platform.StatusFailure, &platform.StatusParams{Operation: "merge-check", Message: fmt.Sprintf("Your pull request is out of date, please rebase against `%s` and resubmit to the merge queue.", c.flagTargetBranch)})
		if err != nil {
			return fmt.Errorf("failed to create merge queue status comment: %w", err)
		}
	}
	return nil
}
