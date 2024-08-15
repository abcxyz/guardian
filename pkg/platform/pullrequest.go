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
// for the current pull request.
func (p *PullRequest) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) error {
	if inputs == nil {
		return fmt.Errorf("inputs cannot be nil")
	}
	if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, inputs.Users, inputs.Teams); err != nil {
		return fmt.Errorf("failed to assign reviewers to pull request: %w", err)
	}
	return nil
}
