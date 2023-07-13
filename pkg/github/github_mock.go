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

	CreatePullRequestCommentsErr error
	UpdatePullRequestCommentsErr error
	DeletePullRequestCommentsErr error
	ListPullRequestCommentsErr   error
}

func (m *MockGitHubClient) CreatePullRequestComment(ctx context.Context, owner, repo string, number int, body string) (int64, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CreatePullRequestComment",
		Params: []any{owner, repo, number, body},
	})

	if m.CreatePullRequestCommentsErr != nil {
		return 0, m.CreatePullRequestCommentsErr
	}

	return 1, nil
}

func (m *MockGitHubClient) UpdatePullRequestComment(ctx context.Context, owner, repo string, id int64, body string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "UpdatePullRequestComment",
		Params: []any{owner, repo, id, body},
	})

	if m.UpdatePullRequestCommentsErr != nil {
		return m.UpdatePullRequestCommentsErr
	}

	return nil
}

func (m *MockGitHubClient) DeletePullRequestComment(ctx context.Context, owner, repo string, id int64) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "DeletePullRequestComment",
		Params: []any{owner, repo, id},
	})

	if m.DeletePullRequestCommentsErr != nil {
		return m.DeletePullRequestCommentsErr
	}

	return nil
}

func (m *MockGitHubClient) ListPullRequestComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListPullRequestComments",
		Params: []any{owner, repo, number},
	})
	return []*github.IssueComment{}, &github.Response{NextPage: 0}, nil
}
