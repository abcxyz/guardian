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

package drift

import (
	"context"
	"fmt"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/internal/version"
)

var _ cli.Command = (*DetectIamDriftCommand)(nil)

type DetectIamDriftCommand struct {
	cli.BaseCommand

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option

	flagOrganizationID        string
	flagGCSBucketQuery        string
	flagDriftignoreFile       string
	flagMaxConcurrentRequests int64
	flagSkipGitHubIssue       bool
	flagGitHubToken           string
	flagGitHubOwner           string
	flagGitHubRepo            string
	flagGitHubIssueLabels     []string
	flagGitHubIssueAssignees  []string
	flagGitHubIssueMentions   []string
}

func (c *DetectIamDriftCommand) Desc() string {
	return `Detect IAM drift in a GCP organization`
}

func (c *DetectIamDriftCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Detect IAM drift in a GCP organization.
`
}

func (c *DetectIamDriftCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "organization-id",
		Target:  &c.flagOrganizationID,
		Example: "123435456456",
		Usage:   `The Google Cloud organization ID for which to detect drift.`,
	})
	f.StringVar(&cli.StringVar{
		Name:    "gcs-bucket-query",
		Target:  &c.flagGCSBucketQuery,
		Example: "labels.terraform:*",
		Usage:   `The label to use to find GCS buckets with Terraform statefiles.`,
	})
	f.StringVar(&cli.StringVar{
		Name:    "driftignore-file",
		Target:  &c.flagDriftignoreFile,
		Example: ".driftignore",
		Usage:   `The driftignore file to use which contains values to ignore.`,
		Default: ".driftignore",
	})
	f.Int64Var(&cli.Int64Var{
		Name:    "max-conncurrent-requests",
		Target:  &c.flagMaxConcurrentRequests,
		Example: "10",
		Usage:   `The maximum number of concurrent requests allowed at any time to GCP.`,
		Default: 10,
	})
	f.BoolVar(&cli.BoolVar{
		Name:    "skip-github-issue",
		Target:  &c.flagSkipGitHubIssue,
		Example: "true",
		Usage:   `Whether or not to create a GitHub Issue when a drift is detected.`,
		Default: false,
	})
	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		Target: &c.flagGitHubToken,
		Usage:  `The github token to use to authenticate to create & manage GitHub Issues.`,
		EnvVar: "GITHUB_TOKEN",
	})
	f.StringVar(&cli.StringVar{
		Name:   "github-owner",
		Target: &c.flagGitHubOwner,
		Usage:  `The github token to use to authenticate to create & manage GitHub Issues.`,
	})
	f.StringVar(&cli.StringVar{
		Name:    "github-repo",
		Target:  &c.flagGitHubRepo,
		Example: "guardian",
		Usage:   `The github token to use to authenticate to create & manage GitHub Issues.`,
	})
	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-issue-assignees",
		Target:  &c.flagGitHubIssueAssignees,
		Example: "dcreey",
		Usage:   `The assignees to assign to for any created GitHub Issues.`,
	})
	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-issue-mentions",
		Target:  &c.flagGitHubIssueMentions,
		Example: "dcreey, my-org/my-team",
		Usage:   `The people or teams to mention on any new drift comments.`,
	})
	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-issue-labels",
		Target:  &c.flagGitHubIssueLabels,
		Example: "guardian-iam-drift",
		Usage:   `The labels to use on any created GitHub Issues.`,
		Default: []string{"guardian-iam-drift"},
	})

	return set
}

func (c *DetectIamDriftCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()

	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	logger := logging.FromContext(ctx)

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	logger.Debugw("Running IAM Drift Detection...",
		"name", version.Name,
		"commit", version.Commit,
		"version", version.Version)

	if c.flagOrganizationID == "" {
		return fmt.Errorf("missing -organization-id")
	}

	iamDriftDetector, err := NewIAMDriftDetector(ctx, c.flagOrganizationID, c.flagMaxConcurrentRequests)
	if err != nil {
		return fmt.Errorf("failed to create iam drift detector: %w", err)
	}

	iamDiff, err := iamDriftDetector.DetectDrift(ctx, c.flagGCSBucketQuery, c.flagDriftignoreFile)
	if err != nil {
		return fmt.Errorf("failed to detect drift: %w", err)
	}

	changesDetected := len(iamDiff.ClickOpsChanges) > 0 || len(iamDiff.MissingTerraformChanges) > 0
	m := driftMessage(iamDiff)
	if changesDetected {
		c.Outf(m)
	}

	if c.flagSkipGitHubIssue {
		return nil
	}
	if changesDetected {
		if err := createOrUpdateIssue(ctx, c.flagGitHubToken, c.flagGitHubOwner, c.flagGitHubRepo, c.flagGitHubIssueAssignees, c.flagGitHubIssueMentions, c.flagGitHubIssueLabels, m); err != nil {
			return fmt.Errorf("failed to create or update GitHub Issue: %w", err)
		}
	} else {
		if err := closeIssues(ctx, c.flagGitHubToken, c.flagGitHubOwner, c.flagGitHubRepo, c.flagGitHubIssueLabels); err != nil {
			return fmt.Errorf("failed to close GitHub Issues: %w", err)
		}
	}

	return nil
}
