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

	IsPullRequest bool
	IncludeTeams  bool

	AssignReviewersErr  error
	ActorUsername       string
	GetPolicyDataErr    error
	DeleteReportErr     error
	ListReportsErr      error
	ModifierContentResp string
	ModifierContentErr  error
	Reports             []*Report
	StoragePrefixResp   string
	StoragePrefixErr    error
	TeamApprovers       []string
	UserApprovers       []string
	UserAccessLevel     string
	UserTeams           []string

	ReportStatusErr             error
	ReportEntrypointsSummaryErr error
	ClearReportsErr             error
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

type MockActorData struct {
	Username    string   `json:"username"`
	AccessLevel string   `json:"access_level"`
	Teams       []string `json:"teams,omitempty"`
}

type MockPolicyData struct {
	Approvers *GetLatestApproversResult `json:"approvers"`
	Actor     *MockActorData            `json:"actor"`
}

func (m *MockPlatform) GetUserRepoPermissions(ctx context.Context) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "GetUserRepoPermissions",
	})

	return m.UserAccessLevel, nil
}

func (m *MockPlatform) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "GetLatestApprovers",
	})

	var teams []string
	if m.IncludeTeams {
		teams = m.TeamApprovers
	}

	return &GetLatestApproversResult{
		Teams: teams,
		Users: m.UserApprovers,
	}, nil
}

func (m *MockPlatform) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "GetTeamMemberships",
	})

	return []string{}, nil
}

func (m *MockPlatform) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs,
		&Request{Name: "GetTeamMemberships"},
		&Request{Name: "GetLatestApprovers"},
		&Request{Name: "GetUserRepoPermissions"},
	)

	if m.GetPolicyDataErr != nil {
		return nil, m.GetPolicyDataErr
	}

	var approverTeams, userTeams []string
	if m.IncludeTeams {
		userTeams = m.UserTeams
		approverTeams = m.TeamApprovers
	}

	var approvers *GetLatestApproversResult
	if m.IsPullRequest {
		approvers = &GetLatestApproversResult{
			Teams: approverTeams,
			Users: m.UserApprovers,
		}
	}

	return &GetPolicyDataResult{
		Mock: &MockPolicyData{
			Approvers: approvers,
			Actor: &MockActorData{
				Username:    m.ActorUsername,
				AccessLevel: m.UserAccessLevel,
				Teams:       userTeams,
			},
		},
	}, nil
}

func (m *MockPlatform) StoragePrefix(ctx context.Context) (string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "StoragePrefix",
	})

	return m.StoragePrefixResp, m.StoragePrefixErr
}

// ReportStatus reports the status of a run.
func (m *MockPlatform) ReportStatus(ctx context.Context, s Status, p *StatusParams) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	// make a copy to prevent outside modification
	pCopy := new(StatusParams)
	*pCopy = *p

	m.Reqs = append(m.Reqs, &Request{
		Name:   "Status",
		Params: []any{s, pCopy},
	})

	return m.ReportStatusErr
}

// ListReports lists existing reports for an issue or change request.
func (m *MockPlatform) ListReports(ctx context.Context, opts *ListReportsOptions) (*ListReportsResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs,
		&Request{Name: "ListReports"},
	)

	if m.ListReportsErr != nil {
		return nil, m.ListReportsErr
	}

	return &ListReportsResult{
		Reports: m.Reports,
	}, nil
}

// DeleteReport deletes an existing comment from an issue or change request.
func (m *MockPlatform) DeleteReport(ctx context.Context, id any) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs,
		&Request{Name: "DeleteReport"},
	)

	if m.DeleteReportErr != nil {
		return m.DeleteReportErr
	}
	return nil
}

// ReportEntrypointsSummary reports the summary for the entrypoints command.
func (m *MockPlatform) ReportEntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	// make a copy to prevent outside modification
	pCopy := new(EntrypointsSummaryParams)
	*pCopy = *p

	m.Reqs = append(m.Reqs, &Request{
		Name:   "EntrypointsSummary",
		Params: []any{pCopy},
	})

	return m.ReportEntrypointsSummaryErr
}

// ClearReports clears any existing reports that can be removed.
func (m *MockPlatform) ClearReports(ctx context.Context) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "Clear",
	})

	return m.ClearReportsErr
}
