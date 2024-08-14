package codereview

import (
	"context"
	"fmt"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/pkg/githubauth"
)

type CodeReview interface {
	AssignReviewers(ctx context.Context, users, teams []string) error
}

type PullRequestParams struct {
	GitHubTokenSource *githubauth.TokenSource

	Owner             string
	Repo              string
	PullRequestNumber int
}

func (p *PullRequestParams) FromGitHubFlags(ctx context.Context, gitHubFlags *flags.GitHubFlags) error {
	tokenSource, err := gitHubFlags.TokenSource(ctx, map[string]string{
		"contents":      "read",
		"pull_requests": "write",
	})
	if err != nil {
		return fmt.Errorf("failed to get token source: %w", err)
	}
	token, err := tokenSource.GitHubToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
}

type PullRequest struct {
	params *PullRequestParams
}

func NewPullRequest(*PullRequestParams) *PullRequest {
	return &PullRequest{}
}

func (p *PullRequest) AssignReviewers(ctx context.Context, users, teams []string) error {
	return nil
}
