// Copyright 2024 The Authors (see AUTHORS file)
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
	"sync"
)

var _ Platform = (*MockPlatform)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockPlatform struct {
	reqMu sync.Mutex
	Reqs  []*Request

	AssignReviewersErr  error
	GetPolicyDataErr    error
	ModifierContentResp string
	ModifierContentErr  error
	StoragePrefixResp   string
	StoragePrefixErr    error
	TeamApprovers       []string
	UserApprovers       []string
}

func (m *MockPlatform) AssignReviewers(ctx context.Context, input *AssignReviewersInput) (*AssignReviewersResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "AssignReviewers",
		Params: []any{input},
	})

	if m.AssignReviewersErr != nil {
		return nil, m.AssignReviewersErr
	}

	return &AssignReviewersResult{
		Teams: input.Teams,
		Users: input.Users,
	}, nil
}

type MockPolicyData struct {
	Approvers *GetLatestApproversResult `json:"approvers"`
}

func (m *MockPlatform) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "GetLatestApprovers",
	})

	return &GetLatestApproversResult{
		Teams: m.TeamApprovers,
		Users: m.UserApprovers,
	}, nil
}

func (m *MockPlatform) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "GetLatestApprovers",
	})

	if m.GetPolicyDataErr != nil {
		return nil, m.GetPolicyDataErr
	}

	return &GetPolicyDataResult{
		Mock: &MockPolicyData{
			Approvers: &GetLatestApproversResult{
				Teams: m.TeamApprovers,
				Users: m.UserApprovers,
			},
		},
	}, nil
}

func (m *MockPlatform) ModifierContent(ctx context.Context) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "ModifierContent",
	})

	return m.ModifierContentResp, m.ModifierContentErr
}

func (m *MockPlatform) StoragePrefix(ctx context.Context) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "StoragePrefix",
	})

	return m.StoragePrefixResp, m.StoragePrefixErr
}
