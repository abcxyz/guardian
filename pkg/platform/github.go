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
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/sethvargo/go-retry"
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
	cfg    *gh.Config
	client *github.Client
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

	g := &GitHub{
		cfg:    cfg,
		client: client,
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
func (g *GitHub) ModifierContent(ctx context.Context) (string, error) {
	if g.cfg.GitHubPullRequestNumber > 0 {
		return g.cfg.GitHubPullRequestBody, nil
	}

	var body strings.Builder
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
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		for _, v := range ghPullRequests {
			body.WriteString(v.GetBody())
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return body.String(), nil
}
