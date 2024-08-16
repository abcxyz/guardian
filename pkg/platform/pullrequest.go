// Copyright 2024 The Authors (see AUTHORS file)
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

package platform

import (
	"context"
	"fmt"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

var _ ChangeRequest = (*PullRequest)(nil)

// PullRequestInput defines the required inputs for creating a new PullRequest
// instance.
type PullRequestInput struct {
	GitHubToken string

	GitHubAppID             string
	GitHubAppPrivateKeyPEM  string
	GitHubAppInstallationID string

	Owner             string
	Repository        string
	PullRequestNumber int
}

// PullRequest is the GitHub implementation for the ChangeRequest interface.
type PullRequest struct {
	client github.GitHub
	params *PullRequestInput
}

// NewPullRequest creates a new PullRequest instance.
func NewPullRequest(ctx context.Context, inputs *PullRequestInput) (*PullRequest, error) {
	tokenSource, err := github.TokenSource(ctx, &github.TokenSourceInputs{
		GitHubToken:             inputs.GitHubToken,
		GitHubAppID:             inputs.GitHubAppID,
		GitHubAppPrivateKeyPEM:  inputs.GitHubAppPrivateKeyPEM,
		GitHubAppInstallationID: inputs.GitHubAppInstallationID,
		GitHubRepo:              inputs.Repository,
		Permissions: map[string]string{
			"contents":      "read",
			"pull_requests": "write",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	client := github.NewClient(ctx, tokenSource)

	return &PullRequest{
		client: client,
		params: inputs,
	}, nil
}

// AssignReviewers calls the GitHub API to assign users and teams as reviewers
// for the current pull request. GitHub's RequestReviewer API will result in
// silent no-op when adding an existing reviewer with new reviewers. Instead,
// we assign every principal individually to avoid the ambiguous API behavior.
func (p *PullRequest) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	logger := logging.FromContext(ctx)
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}

	var result AssignReviewersResult
	for _, u := range inputs.Users {
		if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, []string{u}, nil); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"user", u,
				"error", err,
			)
			continue
		}
		result.Users = append(result.Users, u)
	}
	for _, t := range inputs.Teams {
		if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, nil, []string{t}); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"team", t,
				"error", err,
			)
			continue
		}
		result.Teams = append(result.Teams, t)
	}

	if len(result.Users) == 0 && len(result.Teams) == 0 {
		return nil, fmt.Errorf("failed to assign all requested reviewers to pull request")
	}

	return &result, nil
}
