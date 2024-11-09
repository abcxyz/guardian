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
	"errors"
	"fmt"
	"net/http"
	"slices"
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

// GetUserRepoPermissions returns the repo permission for the user that
// triggered the workflow.
func (g *GitHub) GetUserRepoPermissions(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "querying user repo permissions")

	if g.cfg.GitHubWorkflowUser == "" {
		return "", fmt.Errorf("github-workflow-user is required")
	}
	var result string
	return result, g.withRetries(ctx, func(ctx context.Context) error {
		permissionLevel, resp, err := g.client.Repositories.GetPermissionLevel(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubWorkflowUser)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to get user repo permissions: %w", err)
		}
		result = *permissionLevel.Permission
		return nil
	})
}

// GitHubPolicyData defines the payload of GitHub contextual data used for
// policy evaluation.
type GitHubPolicyData struct {
	PullRequestApprovers *GetLatestApproversResult `json:"pull_request_approvers"`
	UserAccessLevel      string                    `json:"user_access_level"`
}

// GetPolicyData aggregates data from GitHub into a payload used for policy
// evaluation.
func (g *GitHub) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	p, err := g.GetUserRepoPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permissions: %w", err)
	}

	approvers := &GetLatestApproversResult{
		Users: []string{},
		Teams: []string{},
	}
	if g.cfg.GitHubPullRequestNumber > 0 {
		approvers, err = g.GetLatestApprovers(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest approvers: %w", err)
		}
	}

	return &GetPolicyDataResult{
		GitHub: &GitHubPolicyData{
			PullRequestApprovers: approvers,
			UserAccessLevel:      p,
		},
	}, nil
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

// StoragePrefix generates the unique storage prefix for the github platform type.
func (g *GitHub) StoragePrefix(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)

	pullRequestEvents := []string{"pull_request", "pull_request_target"}
	if slices.Contains(pullRequestEvents, g.cfg.GitHubEventName) {
		var merr error
		if g.cfg.GitHubOwner == "" {
			merr = errors.Join(merr, fmt.Errorf("github owner is required for storage"))
		}
		if g.cfg.GitHubRepo == "" {
			merr = errors.Join(merr, fmt.Errorf("github repo is required for storage"))
		}
		if g.cfg.GitHubPullRequestNumber <= 0 {
			merr = errors.Join(merr, fmt.Errorf("github pull request number is required for storage"))
		}

		if merr != nil {
			return "", merr
		}

		logger.DebugContext(ctx, "storage prefix from pull request event data",
			"github_pull_request_number", g.cfg.GitHubPullRequestNumber,
			"github_pull_request_body", g.cfg.GitHubPullRequestBody)

		return fmt.Sprintf("guardian-plans/%s/%s/%d", g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber), nil
	}

	pullRequestFromCommitEvents := []string{"push"}
	if slices.Contains(pullRequestFromCommitEvents, g.cfg.GitHubEventName) {
		logger.DebugContext(ctx, "looking up pull request from commit sha",
			"owner", g.cfg.GitHubOwner,
			"repo", g.cfg.GitHubRepo,
			"commit_sha", g.cfg.GitHubSHA)

		var prData *github.PullRequest
		if err := g.withRetries(ctx, func(ctx context.Context) error {
			ghPullRequests, resp, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubSHA, &github.PullRequestListOptions{
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			})
			if err != nil {
				if resp != nil {
					if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
						return retry.RetryableError(err)
					}
				}
				if resp.StatusCode == http.StatusNotFound {
					return nil
				}
				return fmt.Errorf("failed to list pull requests for commit sha [%s]: %w", g.cfg.GitHubSHA, err)
			}

			if len(ghPullRequests) == 0 {
				return fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
			}

			prData = ghPullRequests[0]

			return nil
		}); err != nil {
			return "", fmt.Errorf("failed to list pull request : %w", err)
		}

		if prData == nil {
			return "", fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
		}

		logger.DebugContext(ctx, "computed pull request number from commit sha",
			"github_pull_request_number", prData.GetNumber())

		return fmt.Sprintf("guardian-plans/%s/%s/%d", g.cfg.GitHubOwner, g.cfg.GitHubRepo, prData.GetNumber()), nil
	}

	logger.DebugContext(ctx, "returning no storage prefix")
	return "", nil
}
