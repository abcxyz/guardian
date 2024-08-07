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

package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/go-github/v53/github"
)

var _ GitHub = (*MockGitHubClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockGitHubClient struct {
	reqMu sync.Mutex
	Reqs  []*Request

	ListRepositoriesErr          error
	ListIssuesErr                error
	CreateIssueErr               error
	CloseIssueErr                error
	CreateIssueCommentsErr       error
	UpdateIssueCommentsErr       error
	DeleteIssueCommentsErr       error
	ListIssueCommentsErr         error
	ListIssueCommentResponse     *IssueCommentResponse
	ListPullRequestsForCommitErr error
	RepoPermissionLevelErr       error
	RepoPermissionLevel          string
	ListJobsForWorkflowRunErr    error
	ResolveJobLogsURLErr         error
	RequestReviewersErr          error
	UserReviewers                []string
	TeamReviewers                []string
}

func (m *MockGitHubClient) ListRepositories(ctx context.Context, owner string, opts *github.RepositoryListByOrgOptions) ([]*Repository, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListRepositories",
		Params: []any{owner, opts},
	})

	if m.ListRepositoriesErr != nil {
		return nil, m.ListRepositoriesErr
	}
	return []*Repository{}, nil
}

func (m *MockGitHubClient) ListIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*Issue, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListIssues",
		Params: []any{owner, repo, opts},
	})

	if m.ListIssuesErr != nil {
		return nil, m.ListIssuesErr
	}
	return []*Issue{}, nil
}

func (m *MockGitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string, assignees, labels []string) (*Issue, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CreateIssue",
		Params: []any{owner, repo, title, body, labels, assignees},
	})

	if m.CreateIssueErr != nil {
		return nil, m.CreateIssueErr
	}

	return &Issue{Number: 1}, nil
}

func (m *MockGitHubClient) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CloseIssue",
		Params: []any{owner, repo, number},
	})

	if m.CloseIssueErr != nil {
		return m.CloseIssueErr
	}

	return nil
}

func (m *MockGitHubClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*IssueComment, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CreateIssueComment",
		Params: []any{owner, repo, number, body},
	})

	if m.CreateIssueCommentsErr != nil {
		return nil, m.CreateIssueCommentsErr
	}

	return &IssueComment{ID: int64(1)}, nil
}

func (m *MockGitHubClient) UpdateIssueComment(ctx context.Context, owner, repo string, id int64, body string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "UpdateIssueComment",
		Params: []any{owner, repo, id, body},
	})

	if m.UpdateIssueCommentsErr != nil {
		return m.UpdateIssueCommentsErr
	}

	return nil
}

func (m *MockGitHubClient) DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "DeleteIssueComment",
		Params: []any{owner, repo, id},
	})

	if m.DeleteIssueCommentsErr != nil {
		return m.DeleteIssueCommentsErr
	}

	return nil
}

func (m *MockGitHubClient) ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) (*IssueCommentResponse, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListIssueComments",
		Params: []any{owner, repo, number},
	})

	if m.ListIssueCommentsErr != nil {
		return nil, m.ListIssueCommentsErr
	}

	if m.ListIssueCommentResponse != nil {
		return m.ListIssueCommentResponse, nil
	}

	return &IssueCommentResponse{}, nil
}

func (m *MockGitHubClient) ListPullRequestsForCommit(ctx context.Context, owner, repo, sha string, opts *github.PullRequestListOptions) (*PullRequestResponse, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListPullRequestsForCommit",
		Params: []any{owner, repo, sha},
	})

	if m.ListPullRequestsForCommitErr != nil {
		return nil, m.ListPullRequestsForCommitErr
	}

	return &PullRequestResponse{
		PullRequests: []*PullRequest{
			{ID: 1, Number: 1},
		},
	}, nil
}

func (m *MockGitHubClient) RepoUserPermissionLevel(ctx context.Context, owner, repo, user string) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "RepoUserPermissionLevel",
		Params: []any{owner, repo, user},
	})

	if m.RepoPermissionLevelErr != nil {
		return "", m.RepoPermissionLevelErr
	}

	return m.RepoPermissionLevel, nil
}

func (m *MockGitHubClient) ListJobsForWorkflowRun(ctx context.Context, owner, repo string, runID int64, opts *github.ListWorkflowJobsOptions) (*JobsResponse, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListJobsForWorkflowRun",
		Params: []any{owner, repo, runID},
	})

	if m.ListJobsForWorkflowRunErr != nil {
		return nil, m.ListJobsForWorkflowRunErr
	}

	return &JobsResponse{
		Jobs: []*Job{
			{ID: 1, Name: "example-job"},
		},
	}, nil
}

func (m *MockGitHubClient) ResolveJobLogsURL(ctx context.Context, jobName, owner, repo string, runID int64) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ResolveJobLogsURL",
		Params: []any{jobName, owner, repo, runID},
	})

	if m.ResolveJobLogsURLErr != nil {
		return "", m.ResolveJobLogsURLErr
	}

	return fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d/job/%d", owner, repo, runID, 1), nil
}

func (m *MockGitHubClient) RequestReviewers(ctx context.Context, owner, repo string, number int, users, teams []string) (*RequestReviewersResponse, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "RequestReviewers",
		Params: []any{owner, repo, number, users, teams},
	})

	if m.RequestReviewersErr != nil {
		return nil, m.RequestReviewersErr
	}

	return &RequestReviewersResponse{
		Users: m.UserReviewers,
		Teams: m.TeamReviewers,
	}, nil
}
