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

	"github.com/abcxyz/pkg/githubauth"
	"github.com/abcxyz/pkg/pointer"
)

var ignoredStatusCodes = map[int]struct{}{
	400: {},
	401: {},
	403: {},
	404: {},
	422: {},
}

// GitHubWorkflowResult is the result status for GitHub workflows.
type GitHubWorkflowResult string

const (
	GitHubWorkflowResultSuccess   = "success"
	GitHubWorkflowResultFailure   = "failure"
	GitHubWorkflowResultCancelled = "cancelled"
	GitHubWorkflowResultSkipped   = "skipped"
)

// Pagination is the paging details for a list response.
type Pagination struct {
	NextPage int
}

// Repository is the GitHub Repository.
type Repository struct {
	ID       int64
	Owner    string
	Name     string
	FullName string
	Topics   []string
}

// Issue is the GitHub Issue.
type Issue struct {
	Number int
}

type IssueComment struct {
	ID   int64
	Body string
}

type IssueCommentResponse struct {
	Comments   []*IssueComment
	Pagination *Pagination
}

type PullRequest struct {
	ID     int64
	Number int
}

type PullRequestResponse struct {
	PullRequests []*PullRequest
	Pagination   *Pagination
}

// Job is the GitHub Job that runs as part of a workflow.
type Job struct {
	ID   int64
	Name string
	URL  string
}

// JobsResponse holds a paginated list of Jobs.
type JobsResponse struct {
	Jobs       []*Job
	Pagination *Pagination
}

// RequestReviewersResponse contains the state of the existing roster of
// requested reviewers on a Pull Request. This will exclude reviewers who
// have approved, left a comment, or requested changes, unless they were
// re-requested for another review.
type RequestReviewersResponse struct {
	Users []string
	Teams []string
}

const (
	Closed = "closed"
	Open   = "open"
	Any    = "any"
)

// GitHub provides the minimum interface for sending requests to the GitHub API.
type GitHub interface {
	//  ListRepositories lists all repositories and returns details about the repositories.
	ListRepositories(ctx context.Context, owner string, opts *github.RepositoryListByOrgOptions) ([]*Repository, error)

	// ListIssues lists all issues and returns their numbers in a repository matching the given criteria.
	ListIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*Issue, error)

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
	ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) (*IssueCommentResponse, error)

	// ListPullRequestsForCommit lists the pull requests associated with a commit.
	ListPullRequestsForCommit(ctx context.Context, owner, repo, sha string, opts *github.PullRequestListOptions) (*PullRequestResponse, error)

	// RepoUserPermissionLevel gets the repository permission level for a user. The possible permissions values
	// are admin, write, read, none.
	RepoUserPermissionLevel(ctx context.Context, owner, repo, user string) (string, error)

	// ListJobsForWorkflowRun lists the jobs for a specific workflow run attempt.
	ListJobsForWorkflowRun(ctx context.Context, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) (*JobsResponse, error)

	// ResolveJobLogsURL finds the direct URL to the logs for a specific workflow run job.
	ResolveJobLogsURL(ctx context.Context, jobName, owner, repo string, runID int64) (string, error)

	// RequestReviewers assigns a list of users and teams as reviewers of a target
	// Pull Request.
	RequestReviewers(ctx context.Context, owner, repo string, number int, users, teams []string) (*RequestReviewersResponse, error)
}

var _ GitHub = (*GitHubClient)(nil)

// GitHubClient implements the GitHub interface.
type GitHubClient struct {
	cfg    *Config
	client *github.Client
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient(ctx context.Context, c *Config) (*GitHubClient, error) {
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.InitialRetryDelay <= 0 {
		c.InitialRetryDelay = 1 * time.Second
	}
	if c.MaxRetryDelay <= 0 {
		c.MaxRetryDelay = 20 * time.Second
	}

	var ts oauth2.TokenSource
	if c.GitHubToken != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: c.GitHubToken,
		})
	} else {
		app, err := githubauth.NewApp(c.GitHubAppID, c.GitHubAppPrivateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, c.GitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		ts = installation.SelectedReposOAuth2TokenSource(ctx, c.Permissions, c.GitHubRepo)
	}

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	g := &GitHubClient{
		cfg:    c,
		client: client,
	}

	return g, nil
}

// NewClient creates a new GitHub client.
// TODO(verbanicm): remove this throughout.
func NewClient(ctx context.Context, ts oauth2.TokenSource, opts ...Option) *GitHubClient {
	cfg := &Config{
		MaxRetries:        3,
		InitialRetryDelay: 1 * time.Second,
		MaxRetryDelay:     20 * time.Second,
	}

	for _, opt := range opts {
		if opt != nil {
			cfg = opt(cfg)
		}
	}

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	g := &GitHubClient{
		cfg:    cfg,
		client: client,
	}

	return g
}

// ListRepositories lists all repositories and returns details about the repositories.
func (g *GitHubClient) ListRepositories(ctx context.Context, owner string, opts *github.RepositoryListByOrgOptions) ([]*Repository, error) {
	pageStart := func(i *int) bool { return i == nil }
	pageEnd := func(i *int) bool { return *i == 0 }
	uniqueResponses := make(map[int64]*Repository)
	opt := *opts

	var page *int // Use nil to indicate start of request.

	// There is a race condition if a repository is created while paginating.
	// It means that a its possible for a repository to appear in the results
	// more than once. We resolve this duplication by using a map to store the
	// responses.
	for pageStart(page) || !pageEnd(page) {
		if err := g.withRetries(ctx, func(ctx context.Context) error {
			if page != nil {
				opt.Page = *page
			}
			repos, resp, err := g.client.Repositories.ListByOrg(ctx, owner, &opt)
			if err != nil {
				if resp != nil {
					if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
						return retry.RetryableError(err)
					}
				}
				return fmt.Errorf("failed to list repositories: %w", err)
			}

			for _, r := range repos {
				uniqueResponses[*r.ID] = &Repository{
					ID:       *r.ID,
					Name:     *r.Name,
					Owner:    *r.Owner.Login,
					FullName: *r.FullName,
					Topics:   r.Topics,
				}
			}
			page = &resp.NextPage
			return nil
		}); err != nil {
			return nil, fmt.Errorf("failed to list repositories after retries: %w", err)
		}
	}

	response := make([]*Repository, 0, len(uniqueResponses))
	for _, r := range uniqueResponses {
		response = append(response, r)
	}

	return response, nil
}

// ListIssues lists all issues and returns their numbers in a repository matching the given criteria.
func (g *GitHubClient) ListIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*Issue, error) {
	pageStart := func(i *int) bool { return i == nil }
	pageEnd := func(i *int) bool { return *i == 0 }
	uniqueResponses := make(map[int]*Issue)
	opt := *opts

	var page *int // Use nil to indicate start of request.

	// There is a race condition if an issue is created while paginating.
	// It means that a its possible for an issue to appear in the results
	// more than once. We resolve this duplication by using a map to store the
	// responses.
	for pageStart(page) || !pageEnd(page) {
		if err := g.withRetries(ctx, func(ctx context.Context) error {
			if page != nil {
				opt.Page = *page
			}
			issues, resp, err := g.client.Issues.ListByRepo(ctx, owner, repo, &opt)
			if err != nil {
				if resp != nil {
					if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
						return retry.RetryableError(err)
					}
				}
				return fmt.Errorf("failed to list issues: %w", err)
			}

			for _, i := range issues {
				number := i.GetNumber()
				uniqueResponses[number] = &Issue{Number: number}
			}
			page = &resp.NextPage
			return nil
		}); err != nil {
			return nil, fmt.Errorf("failed to list issues after retries: %w", err)
		}
	}

	response := make([]*Issue, 0, len(uniqueResponses))
	for _, r := range uniqueResponses {
		response = append(response, r)
	}

	return response, nil
}

// CreateIssue creates an issue.
func (g *GitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string, assignees, labels []string) (*Issue, error) {
	var response *Issue

	req := &github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// GitHub does not accept empty list values.
	if len(assignees) > 0 {
		req.Assignees = &assignees
	}
	if len(labels) > 0 {
		req.Labels = &labels
	}

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		issue, resp, err := g.client.Issues.Create(ctx, owner, repo, req)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to create issue: %w", err)
		}

		response = &Issue{Number: issue.GetNumber()}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to create issue with retries: %w", err)
	}

	return response, nil
}

// CloseIssue closes an issue.
func (g *GitHubClient) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: pointer.To(Closed)})
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to close issue: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to close issue with retries: %w", err)
	}

	return nil
}

// CreateIssueComment creates a comment for an issue or pull request.
func (g *GitHubClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*IssueComment, error) {
	var response *IssueComment

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		comment, resp, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to create pull-request/issue comment: %w", err)
		}
		response = &IssueComment{ID: comment.GetID()}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to create pull-request/issue comment with retries: %w", err)
	}

	return response, nil
}

// UpdateIssueComment updates an issue or pull request comment.
func (g *GitHubClient) UpdateIssueComment(ctx context.Context, owner, repo string, id int64, body string) error {
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.EditComment(ctx, owner, repo, id, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
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
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		resp, err := g.client.Issues.DeleteComment(ctx, owner, repo, id)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
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
func (g *GitHubClient) ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) (*IssueCommentResponse, error) {
	var comments []*IssueComment
	var pagination *Pagination

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghComments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		for _, c := range ghComments {
			comments = append(comments, &IssueComment{ID: c.GetID(), Body: c.GetBody()})
		}

		if resp.NextPage != 0 {
			pagination = &Pagination{NextPage: resp.NextPage}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return &IssueCommentResponse{Comments: comments, Pagination: pagination}, nil
}

// ListPullRequestsForCommit lists the pull requests associated with a commit.
func (g *GitHubClient) ListPullRequestsForCommit(ctx context.Context, owner, repo, sha string, opts *github.PullRequestListOptions) (*PullRequestResponse, error) {
	var pullRequests []*PullRequest
	var pagination *Pagination

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghPullRequests, resp, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, sha, opts)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		for _, c := range ghPullRequests {
			pullRequests = append(pullRequests, &PullRequest{ID: c.GetID(), Number: c.GetNumber()})
		}

		if resp.NextPage != 0 {
			pagination = &Pagination{NextPage: resp.NextPage}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return &PullRequestResponse{PullRequests: pullRequests, Pagination: pagination}, nil
}

// RepoUserPermissionLevel gets the repository permission level for a user. The possible permissions values
// are admin, write, read, none.
func (g *GitHubClient) RepoUserPermissionLevel(ctx context.Context, owner, repo, user string) (string, error) {
	var permissionLevel string

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghPermissionLevel, resp, err := g.client.Repositories.GetPermissionLevel(ctx, owner, repo, user)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to get repository permission level: %w", err)
		}

		permissionLevel = ghPermissionLevel.GetPermission()

		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to get repository permission level: %w", err)
	}

	return permissionLevel, nil
}

func (g *GitHubClient) ListJobsForWorkflowRun(ctx context.Context, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) (*JobsResponse, error) {
	var jobs []*Job
	var pagination *Pagination

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghJobs, resp, err := g.client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to list jobs for workflow run attempt: %w", err)
		}

		for _, workflowJob := range ghJobs.Jobs {
			jobs = append(jobs, &Job{ID: workflowJob.GetID(), Name: workflowJob.GetName(), URL: *workflowJob.HTMLURL})
		}

		if resp.NextPage != 0 {
			pagination = &Pagination{NextPage: resp.NextPage}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list jobs for workflow run attempt: %w", err)
	}

	return &JobsResponse{Jobs: jobs, Pagination: pagination}, nil
}

func (g *GitHubClient) ResolveJobLogsURL(ctx context.Context, jobName, owner, repo string, runID int64) (string, error) {
	jobs, err := g.ListJobsForWorkflowRun(ctx, owner, repo, runID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to resolve direct URL to job logs: %w", err)
	}
	for _, job := range jobs.Jobs {
		if jobName == job.Name {
			return job.URL, nil
		}
	}

	return "", fmt.Errorf("failed to resolve direct URL to job logs: no job found matching name %s", jobName)
}

// RequestReviewers assigns a list of users and teams as reviewers of a target
// Pull Request. Mixing existing reviewers in pending review state with new
// reviewers will result in a no-op without errors thrown.
func (g *GitHubClient) RequestReviewers(ctx context.Context, owner, repo string, number int, users, teams []string) (*RequestReviewersResponse, error) {
	var assigned *RequestReviewersResponse
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		pr, resp, err := g.client.PullRequests.RequestReviewers(ctx, owner, repo, number, github.ReviewersRequest{
			Reviewers:     users,
			TeamReviewers: teams,
		})
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to request reviewers: %w", err)
		}

		for _, u := range pr.RequestedReviewers {
			assigned.Users = append(assigned.Users, *u.Login)
		}
		for _, t := range pr.RequestedTeams {
			assigned.Teams = append(assigned.Teams, *t.Slug)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to request reviewers: %w", err)
	}
	return assigned, nil
}

func (g *GitHubClient) withRetries(ctx context.Context, retryFunc retry.RetryFunc) error {
	backoff := retry.NewFibonacci(g.cfg.InitialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.MaxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.MaxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, retryFunc); err != nil {
		return fmt.Errorf("failed to execute retriable function: %w", err)
	}
	return nil
}

type TokenSourceInputs struct {
	GitHubToken string

	GitHubAppID             string
	GitHubAppPrivateKeyPEM  string
	GitHubAppInstallationID string
	GitHubRepo              string
	Permissions             map[string]string
}

// TokenSource creates a token source from a GitHub token or GitHub App used for
// authenticating a github client.
func TokenSource(ctx context.Context, inputs *TokenSourceInputs) (oauth2.TokenSource, error) {
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}

	if inputs.GitHubToken != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: inputs.GitHubToken,
		}), nil
	}

	app, err := githubauth.NewApp(inputs.GitHubAppID, inputs.GitHubAppPrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create github app token source: %w", err)
	}

	installation, err := app.InstallationForID(ctx, inputs.GitHubAppInstallationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get github app installation: %w", err)
	}
	return installation.SelectedReposOAuth2TokenSource(ctx, inputs.Permissions, inputs.GitHubRepo), nil
}
