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

	"github.com/abcxyz/guardian/pkg/util"
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

	CreateIssueCommentsErr error
	UpdateIssueCommentsErr error
	DeleteIssueCommentsErr error
	ListIssueCommentsErr   error
}

func (m *MockGitHubClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*github.IssueComment, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CreateIssueComment",
		Params: []any{owner, repo, number, body},
	})

	if m.CreateIssueCommentsErr != nil {
		return nil, m.CreateIssueCommentsErr
	}

	id := util.Ptr[int64](1)

	return &github.IssueComment{ID: id}, nil
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

func (m *MockGitHubClient) ListIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ListIssueComments",
		Params: []any{owner, repo, number},
	})
	return []*github.IssueComment{}, &github.Response{NextPage: 0}, nil
}
