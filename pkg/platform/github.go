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

package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/sethvargo/go-retry"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	gh "github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/githubauth"
	"github.com/abcxyz/pkg/logging"
)

var (
	_ Platform = (*GitHub)(nil)

	// ignoredStatusCodes are status codes that should not be retried. This list
	// is taken from the GitHub REST API documentation.
	ignoredStatusCodes = map[int]struct{}{
		403: {},
		422: {},
	}
)

// GitHub implements the Platform interface.
type GitHub struct {
	cfg           *gh.Config
	client        *github.Client
	graphqlClient *githubv4.Client
}

// NewGitHub creates a new GitHub client.
func NewGitHub(ctx context.Context, cfg *gh.Config) (*GitHub, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialRetryDelay <= 0 {
		cfg.InitialRetryDelay = 1 * time.Second
	}
	if cfg.MaxRetryDelay <= 0 {
		cfg.MaxRetryDelay = 20 * time.Second
	}

	ghToken := cfg.GuardianGitHubToken
	if ghToken == "" {
		ghToken = cfg.GitHubToken
	}

	var ts oauth2.TokenSource
	if ghToken != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: ghToken,
		})
	} else {
		app, err := githubauth.NewApp(cfg.GitHubAppID, cfg.GitHubAppPrivateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, cfg.GitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		ts = installation.SelectedReposOAuth2TokenSource(ctx, cfg.Permissions, cfg.GitHubRepo)
	}

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	graphqlClient := githubv4.NewClient(tc)

	g := &GitHub{
		cfg:           cfg,
		client:        client,
		graphqlClient: graphqlClient,
	}

	return g, nil
}

// RequestReviewers abstracts GitHub's RequestReviewers API with retries.
func (g *GitHub) requestReviewers(ctx context.Context, reviewers *github.ReviewersRequest) error {
	if reviewers == nil {
		return fmt.Errorf("reviewer cannot be nil")
	}
	return g.withRetries(ctx, func(ctx context.Context) error {
		if _, resp, err := g.client.PullRequests.RequestReviewers(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber, *reviewers); err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to request reviewers: %w", err)
		}
		return nil
	})
}

// AssignReviewers assigns a list of users and teams as reviewers of a target
// Pull Request. The implementation assigns one principal per request because
// mixing existing reviewers in pending review state with new reviewers will
// result in a no-op without errors thrown.
func (g *GitHub) AssignReviewers(ctx context.Context, input *AssignReviewersInput) (*AssignReviewersResult, error) {
	logger := logging.FromContext(ctx)
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	var result AssignReviewersResult
	for _, u := range input.Users {
		if err := g.requestReviewers(ctx, &github.ReviewersRequest{Reviewers: []string{u}}); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"user", u,
				"error", err,
			)
			continue
		}
		result.Users = append(result.Users, u)
	}
	for _, t := range input.Teams {
		if err := g.requestReviewers(ctx, &github.ReviewersRequest{TeamReviewers: []string{t}}); err != nil {
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

type pullRequestReview struct {
	Author struct {
		Login string
	}
	State string
}

type latestApproverQuery struct {
	Repository struct {
		PullRequest struct {
			LatestReviews struct {
				Nodes []pullRequestReview
			} `graphql:"latestReviews(first: 100)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type member struct {
	Login string
}
type team struct {
	Name    string
	Members struct {
		Nodes []member
	} `graphql:"members(first: 100)"`
}
type organizationTeamsAndMembershipsQuery struct {
	Organization struct {
		Teams struct {
			Nodes []team
		} `graphql:"teams(first: 100)"`
	} `graphql:"organization(login: $owner)"`
}

// GetLatestApprovers retrieves the users whose latest review for a pull request
// is an approval. It also returns the teams and subteams that the user
// approvers are members of to indicate approval on behalf of those teams. Note,
// a comment following a previous approval by the same user will still keep the
// APPROVED state. However, if a reviewer previously approved the PR and
// requests changes/dismisses the review, then the approval is not counted.
func (g *GitHub) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "querying latest approvers")

	var approversQuery latestApproverQuery
	if err := g.graphqlClient.Query(ctx, &approversQuery, map[string]any{
		"owner":  githubv4.String(g.cfg.GitHubOwner),
		"repo":   githubv4.String(g.cfg.GitHubRepo),
		"number": githubv4.Int(g.cfg.GitHubPullRequestNumber),
	}); err != nil {
		return nil, fmt.Errorf("failed to query latest approvers: %w", err)
	}

	// Explicitly sets the default Users and Teams to empty slices. If these are
	// not explicitly provided to OPA, then the policy result may be incorrect.
	result := &GetLatestApproversResult{
		Teams: []string{},
		Users: []string{},
	}
	hasApproved := make(map[string]struct{}, len(approversQuery.Repository.PullRequest.LatestReviews.Nodes))
	for _, review := range approversQuery.Repository.PullRequest.LatestReviews.Nodes {
		if review.State == "APPROVED" {
			result.Users = append(result.Users, review.Author.Login)
			hasApproved[review.Author.Login] = struct{}{}
		}
	}

	var teamQuery organizationTeamsAndMembershipsQuery
	if err := g.graphqlClient.Query(ctx, &teamQuery, map[string]any{
		"owner": githubv4.String(g.cfg.GitHubOwner),
	}); err != nil {
		return nil, fmt.Errorf("failed to query organization teams and memberships: %w", err)
	}

	for _, team := range teamQuery.Organization.Teams.Nodes {
		for _, member := range team.Members.Nodes {
			if _, ok := hasApproved[member.Login]; ok {
				result.Teams = append(result.Teams, team.Name)
				break
			}
		}
	}

	logger.DebugContext(ctx, "found latest approvers from",
		"users", result.Users,
		"teams", result.Teams,
	)

	return result, nil
}

func (g *GitHub) withRetries(ctx context.Context, retryFunc retry.RetryFunc) error {
	backoff := retry.NewFibonacci(g.cfg.InitialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.MaxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.MaxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, retryFunc); err != nil {
		return fmt.Errorf("failed to execute retriable function: %w", err)
	}
	return nil
}

// ModifierContent returns the pull request body as the content to parse modifiers
// from.
func (g *GitHub) ModifierContent(ctx context.Context) string {
	return g.cfg.GitHubPullRequestBody
}
