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

var ignoredStatusCodes = map[int]struct{}{
	400: {},
	401: {},
	403: {},
	404: {},
	422: {},
}

// Config is the config values for the GitHub client.
type Config struct {
	maxRetries        uint64
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
}

var (
	Closed = "closed"
	Open   = "open"
	Any    = "any"
)

// GitHub provides the minimum interface for sending requests to the GitHub API.
type GitHub interface {
	// ListIssues lists all issues and returns their numbers in a repository matching the given criteria.
	ListIssues(ctx context.Context, owner, repo string, labels []string, state string) ([]*Issue, error)

	// CreateIssue creates an issue.
	CreateIssue(ctx context.Context, owner, repo, title, body string, assignees, labels []string) (*Issue, error)

	// CloseIssue closes an issue.
	CloseIssue(ctx context.Context, owner, repo string, number int) error

	// CreateIssueComment creates a comment for an issue or pull request.
	CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*IssueComment, error)

	// UpdateIssueComment updates an issue or pull request comment.
	UpdateIssueComment(ctx context.Context, owner, repo string, id int64, body string) error

	// DeleteIssueComment deletes an issue or pull request comment.
	DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error

	// ListIssueComments lists existing comments for an issue or pull request.
	ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*int64, error)
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

	// TODO(#130): support multiple authentication methods
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

type Issue struct {
	Number *int
}

// ListIssues lists all issues and returns their numbers in a repository matching the given criteria.
func (g *GitHubClient) ListIssues(ctx context.Context, owner, repo string, labels []string, state string) ([]*Issue, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	var response []*Issue

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		issues, resp, err := g.client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			Labels: labels,
			State:  state,
		})
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("failed to list issues: %w", err)
		}

		for _, i := range issues {
			response = append(response, &Issue{Number: i.Number})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list issues after retries: %w", err)
	}

	return response, nil
}

// CreateIssue creates an issue.
func (g *GitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string, assignees, labels []string) (*Issue, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	var response *Issue

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		issue, resp, err := g.client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
			Title:     &title,
			Body:      &body,
			Labels:    &labels,
			Assignees: &assignees,
		})
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("failed to create pull request comment: %w", err)
		}

		response = &Issue{Number: issue.Number}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to create pull request comment: %w", err)
	}

	return response, nil
}

// CloseIssue closes an issue.
func (g *GitHubClient) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: &Closed})
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("failed to create pull request comment: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to create pull request comment: %w", err)
	}

	return nil
}

type IssueComment struct {
	ID *int64
}

// CreateIssueComment creates a comment for an issue or pull request.
func (g *GitHubClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*IssueComment, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	var response *IssueComment

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		comment, resp, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("failed to create pull request comment: %w", err)
		}
		response = &IssueComment{ID: comment.ID}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to create pull request comment: %w", err)
	}

	return response, nil
}

// UpdateIssueComment updates an issue or pull request comment.
func (g *GitHubClient) UpdateIssueComment(ctx context.Context, owner, repo string, id int64, body string) error {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.EditComment(ctx, owner, repo, id, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
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

// DeleteIssueComment deletes an issue or pull request comment.
func (g *GitHubClient) DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		resp, err := g.client.Issues.DeleteComment(ctx, owner, repo, id)
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
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

// ListIssueComments lists existing comments for an issue or pull request.
func (g *GitHubClient) ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*int64, error) {
	backoff := retry.NewFibonacci(g.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.maxRetryDelay, backoff)

	var commentsResponse []*int64

	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		comments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
				return retry.RetryableError(err)
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		for _, c := range comments {
			commentsResponse = append(commentsResponse, c.ID)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return commentsResponse, nil
}
