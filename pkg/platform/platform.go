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

// Package platform defines interfaces for interacting with code review
// platforms.
package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v53/github"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

const (
	TypeUnspecified = ""
	TypeLocal       = "local"
	TypeGitHub      = "github"
	TypeGitLab      = "gitlab"
)

var (
	allowedTypes = map[string]struct{}{
		TypeLocal:  {},
		TypeGitHub: {},
		TypeGitLab: {},
	}
	// SortedTypes are the sorted Platform types for printing messages and prediction.
	SortedTypes = func() []string {
		allowed := append([]string{}, TypeLocal, TypeGitHub, TypeGitLab)
		sort.Strings(allowed)
		return allowed
	}()

	_ Platform = (*GitHub)(nil)
)

// AssignReviewersInput defines the principal types that can be assigned to a
// change request.
type AssignReviewersInput struct {
	Users []string
	Teams []string
}

// AssignReviewersResult contains the principals that were successfully assigned
// to a change request.
type AssignReviewersResult struct {
	Users []string
	Teams []string
}

// GetLatestApproversResult contains the reviewers whose latest review is an
// approval.
type GetLatestApproversResult struct {
	Users []string `json:"users"`
	Teams []string `json:"teams,omitempty"`
}

// GetPolicyDataResult contains the required data for policy evaluation, by
// platform.
type GetPolicyDataResult struct {
	GitHub *GitHubPolicyData `json:"github,omitempty"`
	Mock   *MockPolicyData   `json:"mock,omitempty"`
}

// Report is a comment/note on an issue or change request.
type Report struct {
	ID   any
	Body string
}

// ListReportsOptions contains the options for listing reports.
type ListReportsOptions struct {
	GitHub *github.IssueListCommentsOptions
	GitLab *gitlab.ListMergeRequestNotesOptions
}

// ListReportsResults contains the results of listing reports.
type ListReportsResult struct {
	Reports    []*Report
	Pagination *Pagination
}

// ListChangeRequestsByCommitOptions contains embedded client options for each
// platform.
type ListChangeRequestsByCommitOptions struct {
	GitHub *github.PullRequestListOptions
}

// ListChangeRequestsByCommitResponse contains the changes requests and
// pagination options return by the platform.
type ListChangeRequestsByCommitResponse struct {
	PullRequests []*PullRequest
	Pagination   *Pagination
}

// Pagination is the paging details for a list response.
type Pagination struct {
	NextPage int
}

// Platform defines the minimum interface for a code review platform.
type Platform interface {
	// AssignReviewers assigns principals to review a change request.
	AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error)

	// GetUserRepoPermissions returns a user's access level to a repository.
	GetUserRepoPermissions(ctx context.Context) (string, error)

	// GetLatestApprovers retrieves the reviewers whose latest review is an
	// approval.
	GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error)

	// GetUserTeamMemberships retrieves a list of teams that a user is a member
	// of, within the given organization.
	GetUserTeamMemberships(ctx context.Context, username string) ([]string, error)

	// GetPolicyData retrieves the required data for policy evaluation.
	GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error)

	// ListReports lists existing reports for an issue or change request.
	ListReports(ctx context.Context, opts *ListReportsOptions) (*ListReportsResult, error)

	// ListChangeRequestsByCommit lists the change requests associated with a commit SHA.
	ListChangeRequestsByCommit(ctx context.Context, sha string, opts *ListChangeRequestsByCommitOptions) (*ListChangeRequestsByCommitResponse, error)

	// DeleteReport deletes an existing comment from an issue or change request.
	DeleteReport(ctx context.Context, id any) error

	// ModifierContent returns the modifier content for parsing modifier flags.
	ModifierContent(ctx context.Context) (string, error)

	// StoragePrefix generates the unique storage prefix for the platform type.
	StoragePrefix(ctx context.Context) (string, error)

	// ReportStatus reports the status of a run.
	ReportStatus(ctx context.Context, status Status, params *StatusParams) error

	// ReportEntrypointsSummary reports the summary for the entrypoints command.
	ReportEntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error

	// ClearReports clears any existing reports that can be removed.
	ClearReports(ctx context.Context) error
}

// NewPlatform creates a new platform based on the provided type.
func NewPlatform(ctx context.Context, cfg *Config) (Platform, error) {
	if strings.EqualFold(cfg.Type, TypeLocal) {
		return NewLocal(ctx, &cfg.Local), nil
	}

	if strings.EqualFold(cfg.Type, TypeGitHub) {
		gc, err := NewGitHub(ctx, &cfg.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github: %w", err)
		}
		return gc, nil
	}

	if strings.EqualFold(cfg.Type, TypeGitLab) {
		gl, err := NewGitLab(ctx, &cfg.GitLab)
		if err != nil {
			return nil, fmt.Errorf("failed to create gitlab: %w", err)
		}
		return gl, nil
	}
	return nil, fmt.Errorf("unknown platform type: %s", cfg.Type)
}
