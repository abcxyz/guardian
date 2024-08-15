package codereview

import (
	"context"
	"fmt"

	"github.com/abcxyz/guardian/pkg/github"
)

var _ CodeReview = (*PullRequest)(nil)

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

// PullRequest is the GitHub implementation for the CodeReview interface.
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
	if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, inputs.Users, inputs.Teams); err != nil {
		return fmt.Errorf("failed to assign reviewers to pull request: %w", err)
	}
	return nil
}
