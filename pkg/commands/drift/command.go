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
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/internal/version"
	driftflags "github.com/abcxyz/guardian/pkg/commands/drift/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ cli.Command = (*DetectIamDriftCommand)(nil)

const (
	issueTitle = "IAM drift detected"
	issueBody  = `We've detected a drift between your submitted IAM policies and actual
        IAM policies.

        See the comment(s) below to see details of the drift

        Please determine which parts are correct, and submit updated
        terraform config and/or remove the extra policies.

        Re-run drift detection manually once complete to verify all diffs are properly resolved.`
)

// DetectIamDriftCommand is a subcommand for Guardian that enables detecting IAM drift.
type DetectIamDriftCommand struct {
	cli.BaseCommand

	githubConfig github.Config

	driftflags.DriftIssueFlags

	flagOrganizationID        string
	flagGCSBucketQuery        string
	flagDriftignoreFile       string
	flagMaxConcurrentRequests int64
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
	set := c.NewFlagSet()

	c.githubConfig.RegisterFlags(set)
	c.Register(set)

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

	set.AfterParse(func(existingErr error) (merr error) {
		if len(c.FlagGitHubIssueLabels) == 0 {
			c.FlagGitHubIssueLabels = []string{"guardian-iam-drift"}
		}
		return merr
	})

	return set
}

func (c *DetectIamDriftCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_iam_detect_drift", 1)

	f := c.Flags()

	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	logger := logging.FromContext(ctx)

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	logger.DebugContext(ctx, "running IAM Drift Detection",
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

	if c.FlagSkipGitHubIssue {
		return nil
	}
	if c.FlagGitHubCommentMessageAppend != "" {
		m = strings.Join([]string{m, c.FlagGitHubCommentMessageAppend}, "\n\n")
	}
	githubClient, err := github.NewGitHubClient(ctx, &c.githubConfig)
	if err != nil {
		return fmt.Errorf("failed to create github client: %w", err)
	}

	issueService := NewGitHubDriftIssueService(
		githubClient,
		c.githubConfig.GitHubOwner,
		c.githubConfig.GitHubRepo,
		issueTitle,
		issueBody,
	)
	if changesDetected {
		if err := issueService.CreateOrUpdateIssue(ctx, c.FlagGitHubIssueAssignees, c.FlagGitHubIssueLabels, m); err != nil {
			return fmt.Errorf("failed to create or update GitHub Issue: %w", err)
		}
	} else {
		if err := issueService.CloseIssues(ctx, c.FlagGitHubIssueLabels); err != nil {
			return fmt.Errorf("failed to close GitHub Issues: %w", err)
		}
	}

	return nil
}

func driftMessage(drift *IAMDrift) string {
	var msg strings.Builder
	coKeys := maps.Keys(drift.ClickOpsChanges)
	mtKeys := maps.Keys(drift.MissingTerraformChanges)
	sort.Strings(coKeys)
	sort.Strings(mtKeys)
	if len(coKeys) > 0 {
		msg.WriteString("Found Click Ops Changes (IAM resources actually present in GCP but not described in terraform state)\n")
		msg.WriteString("| ID | Resource ID | Member | Role |\n")
		msg.WriteString("|----|-------------|--------|------|\n")
		for _, k := range coKeys {
			coChange := drift.ClickOpsChanges[k]
			msg.WriteString(fmt.Sprintf("|%s|%s|%s|%s|\n", k, ResourceURI(coChange), coChange.Member, coChange.Role))
		}
		if len(mtKeys) > 0 {
			msg.WriteString("\n\n")
		}
	}
	if len(mtKeys) > 0 {
		msg.WriteString("Found Missing Terraform Changes (IAM resources not actually present in GCP but still described in terraform state)\n")
		msg.WriteString("| ID | StateFile URI | Resource ID | Member | Role |\n")
		msg.WriteString("|----|---------------|-------------|--------|------|\n")
		for _, k := range mtKeys {
			mtChange := drift.MissingTerraformChanges[k]
			msg.WriteString(fmt.Sprintf("|%s|%s|%s|%s|%s|\n", k, mtChange.StateFileURI, ResourceURI(mtChange.AssetIAM), mtChange.Member, mtChange.AssetIAM.Role))
		}
	}
	return msg.String()
}
