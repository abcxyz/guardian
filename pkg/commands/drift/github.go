// Copyright 2023 Google LLC
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

	"github.com/abcxyz/guardian/pkg/github"
)

const (
	issueTitle = "IAM drift detected"
	issueBody  = `We've detected a drift between your submitted IAM policies and actual
		IAM policies.

		See the comment(s) below to see details of the drift

		Please determine which parts are correct, and submit updated
		terraform config and/or remove the extra policies.

		Re-run drift detection manually once complete to verify all diffs are properly resolved.`
)

func createUpdateOrCloseIssues(ctx context.Context, token, owner, repo string, assignees, labels []string, drift *IAMDrift) error {
	if len(labels) == 0 {
		return fmt.Errorf("invalid argument - at least one 'label' must be provided")
	}

	changesDetected := len(drift.ClickOpsChanges) > 0 || len(drift.MissingTerraformChanges) > 0
	gh := github.NewClient(ctx, token)

	issues, err := gh.ListIssues(ctx, owner, repo, labels, github.Open)
	if err != nil {
		return fmt.Errorf("failed to list GitHub issues for %s/%s: %w", owner, repo, err)
	}
	// Close all open issues matching the given labels if the drift is resolved.
	if !changesDetected && len(issues) > 0 {
		for _, issueToClose := range issues {
			_, err = gh.CreateIssueComment(ctx, owner, repo, *issueToClose.Number, "Drift Resolved.")
			if err != nil {
				return fmt.Errorf("failed to comment on issue %s/%s %d: %w", owner, repo, issueToClose.Number, err)
			}
			err = gh.CloseIssue(ctx, owner, repo, *issueToClose.Number)
			if err != nil {
				return fmt.Errorf("failed to close GitHub issue for %s/%s %d: %w", owner, repo, issueToClose.Number, err)
			}
		}
		return nil
	}

	var issueNumber int
	if len(issues) == 0 {
		issue, err := gh.CreateIssue(ctx, owner, repo, issueTitle, issueBody, assignees, labels)
		if err != nil {
			return fmt.Errorf("failed to create GitHub for %s/%s: %w", owner, repo, err)
		}
		issueNumber = *issue.Number
	} else {
		issueNumber = *issues[0].Number
	}

	_, err = gh.CreateIssueComment(ctx, owner, repo, issueNumber, driftMessage(drift))
	if err != nil {
		return fmt.Errorf("failed to comment on issue %s/%s %d: %w", owner, repo, issueNumber, err)
	}

	return nil
}

func driftMessage(drift *IAMDrift) string {
	var msg string
	if len(drift.ClickOpsChanges) > 0 {
		uris := keys(drift.ClickOpsChanges)
		sort.Strings(uris)
		msg += fmt.Sprintf("Found Click Ops Changes \n> %s", strings.Join(uris, "\n> "))
		if len(drift.MissingTerraformChanges) > 0 {
			msg += "\n\n"
		}
	}
	if len(drift.MissingTerraformChanges) > 0 {
		uris := keys(drift.MissingTerraformChanges)
		sort.Strings(uris)
		msg += fmt.Sprintf("Found Missing Terraform Changes \n> %s", strings.Join(uris, "\n> "))
	}
	return msg
}
