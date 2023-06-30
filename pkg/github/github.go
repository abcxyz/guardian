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

// Package github provides the functionality to send requests to the GitHub API.
package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/sethvargo/go-retry"
	"golang.org/x/oauth2"
)

var ignoredStatusCodes = []int{400, 401, 403, 404, 422}

type Config struct {
	maxRetries        uint64
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
}

// GitHub provides the minimum interface for sending requests to the GitHub API.
type GitHub interface {
	CreatePullRequestComment(ctx context.Context, owner, repo string, number int, body string) (int64, error)
	UpdatePullRequestComment(ctx context.Context, owner, repo string, id int64, body string) error
	DeletePullRequestComment(ctx context.Context, owner, repo string, id int64) error
	ListPullRequestComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error)
}

var _ GitHub = (*GitHubClient)(nil)

// GitHubClient implements the GitHub interface.
type GitHubClient struct {
	cfg    *Config
	client *github.Client
}

// NewClient creates a new GitHub client.
func NewClient(ctx context.Context, token string, opts ...Option) *GitHubClient {
	cfg := &Config{
		maxRetries:        3,
		initialRetryDelay: 1 * time.Second,
		maxRetryDelay:     20 * time.Second,
	}

	for _, opt := range opts {
		if opt != nil {
			cfg = opt(cfg)
		}
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	g := &GitHubClient{
		cfg:    cfg,
		client: client,
	}

	return g
}

// CreatePullRequestComment creates a pull request comment.
func (g *GitHubClient) CreatePullRequestComment(ctx context.Context, owner, repo string, number int, body string) (int64, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	var commentID int64

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		comment, resp, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if resp != nil && !containsInt(ignoredStatusCodes, resp.StatusCode) {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("failed to create pull request comment: %w", err)
		}

		commentID = comment.GetID()
		return nil
	}); err != nil {
		return 0, fmt.Errorf("failed to create pull request comment: %w", err)
	}

	return commentID, nil
}

// UpdatePullRequestComment updates a pull request comment.
func (g *GitHubClient) UpdatePullRequestComment(ctx context.Context, owner, repo string, id int64, body string) error {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.EditComment(ctx, owner, repo, id, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if resp != nil && !containsInt(ignoredStatusCodes, resp.StatusCode) {
				return retry.RetryableError(err)
			}
			return fmt.Errorf("failed to update pull request comment: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update pull request comment: %w", err)
	}

	return nil
}

// DeletePullRequestComment deletes a pull request comment.
func (g *GitHubClient) DeletePullRequestComment(ctx context.Context, owner, repo string, id int64) error {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		resp, err := g.client.Issues.DeleteComment(ctx, owner, repo, id)
		if err != nil {
			if resp != nil && !containsInt(ignoredStatusCodes, resp.StatusCode) {
				return retry.RetryableError(err)
			}
			return fmt.Errorf("failed to delete pull request comment: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to delete pull request comment: %w", err)
	}

	return nil
}

// ListPullRequestComments lists existing comments for a pull request.
func (g *GitHubClient) ListPullRequestComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	commentsResponse := make([]*github.IssueComment, 0)
	var gitHubResponse *github.Response

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		comments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			if resp != nil && !containsInt(ignoredStatusCodes, resp.StatusCode) {
				return retry.RetryableError(err)
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		commentsResponse = comments
		gitHubResponse = resp

		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return commentsResponse, gitHubResponse, nil
}

// containsInt is a helper function to determine if a slice contains an integer.
func containsInt(search []int, value int) bool {
	for _, target := range search {
		if target == value {
			return true
		}
	}
	return false
}
