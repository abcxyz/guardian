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
	"strings"

	"github.com/abcxyz/guardian/pkg/config"
)

var _ Platform = (*GitHub)(nil)

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

	// StoragePrefix generates the unique storage prefix for the platform type.
	StoragePrefix(ctx context.Context) (string, error)
}

// NewPlatform creates a new platform based on the provided type.
func NewPlatform(ctx context.Context, cfg *config.Platform) (Platform, error) {
	if strings.EqualFold(cfg.Type, config.TypeLocal) {
		return NewLocal(ctx, &cfg.Local), nil
	}

	if strings.EqualFold(cfg.Type, config.TypeGitHub) {
		gc, err := NewGitHub(ctx, &cfg.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github: %w", err)
		}
		return gc, nil
	}

	if strings.EqualFold(cfg.Type, config.TypeGitLab) {
		gl, err := NewGitLab(ctx, &cfg.GitLab)
		if err != nil {
			return nil, fmt.Errorf("failed to create gitlab: %w", err)
		}
		return gl, nil
	}
	return nil, fmt.Errorf("unknown platform type: %s", cfg.Type)
}
