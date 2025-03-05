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
)

var _ Platform = (*Local)(nil)

// Local implements the Platform interface for running Guardian locally.
type Local struct {
	cfg *localConfig
}

type localConfig struct {
	LocalModifierContent string
}

// NewLocal creates a new Local instance.
func NewLocal(ctx context.Context, cfg *localConfig) *Local {
	return &Local{cfg: cfg}
}

// AssignReviewers is a no-op.
func (l *Local) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	return &AssignReviewersResult{}, nil
}

// GetUserRepoPermissions is a no-op and returns an empty string.
func (l *Local) GetUserRepoPermissions(ctx context.Context) (string, error) {
	return "", nil
}

// GetUserTeamMemberships is a no-op and returns an empty slice.
func (l *Local) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	return []string{}, nil
}

// GetLatestApprovers returns an empty result.
func (l *Local) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	return &GetLatestApproversResult{}, nil
}

// GetPolicyData returns an empty result.
func (l *Local) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	return &GetPolicyDataResult{}, nil
}

// ModifierContent returns an empty string. Local runs would have more context and
// users can pass in the necessary values instead of relying on modifiers.
func (l *Local) ModifierContent(ctx context.Context) (string, error) {
	return "", nil
}

// StoragePrefix returns an empty string for the local platform type.
func (l *Local) StoragePrefix(ctx context.Context) (string, error) {
	return "", nil
}

// ListReports lists existing reports for an issue or change request.
func (l *Local) ListReports(ctx context.Context, opts *ListReportsOptions) (*ListReportsResult, error) {
	return nil, nil
}

// DeleteReport deletes an existing comment from an issue or change request.
func (l *Local) DeleteReport(ctx context.Context, id any) error {
	return nil
}

// ReportStatus is a no-op.
func (l *Local) ReportStatus(ctx context.Context, status Status, params *StatusParams) error {
	return nil
}

// ReportEntrypointsSummary is a no-op.
func (l *Local) ReportEntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error {
	return nil
}

// ClearReports clears any existing reports that can be removed.
func (l *Local) ClearReports(ctx context.Context) error {
	return nil
}

// ListChangeRequestsByCommit is a no-op.
func (l *Local) ListChangeRequestsByCommit(ctx context.Context, sha string, opts *ListChangeRequestsByCommitOptions) (*ListChangeRequestsByCommitResponse, error) {
	return nil, nil
}
