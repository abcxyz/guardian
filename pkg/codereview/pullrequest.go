package codereview

import (
	"context"
	"errors"
	"fmt"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

type PullRequestInput struct {
	GitHubToken string

	GitHubAppID             string
	GitHubAppPrivateKeyPEM  string
	GitHubAppInstallationID string

	Owner             string
	Repository        string
	PullRequestNumber int
}

type PullRequest struct {
	client github.GitHub
	params *PullRequestInput
}

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
// for the current pull request. This makes a request per user and team to avoid
// an uncaught behavior from the GitHub API, which does not assign any of the
// provided reviewers if any of the principals exist on the pending review list.
func (p *PullRequest) AssignReviewers(ctx context.Context, users, teams []string) error {
	logger := logging.FromContext(ctx)

	var hasSucceeded bool
	var merr error
	for _, u := range users {
		if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, []string{u}, nil); err != nil {
			logger.ErrorContext(ctx, "failed to request review",
				"user", u,
				"error", err)
			merr = errors.Join(merr, fmt.Errorf("failed to request review for team '%s': %w", u, err))
			continue
		}
		hasSucceeded = true
	}

	for _, t := range teams {
		if _, err := p.client.RequestReviewers(ctx, p.params.Owner, p.params.Repository, p.params.PullRequestNumber, nil, []string{t}); err != nil {
			logger.ErrorContext(ctx, "failed to request review",
				"team", t,
				"error", err)
			merr = errors.Join(merr, fmt.Errorf("failed to request review for team '%s': %w", t, err))
			continue
		}
		hasSucceeded = true
	}

	if hasSucceeded {
		return nil
	}

	return merr
}
