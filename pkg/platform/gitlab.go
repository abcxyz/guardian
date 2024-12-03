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

package platform

import (
	"context"

	"github.com/abcxyz/guardian/pkg/platform/config"
)

var _ Platform = (*GitLab)(nil)

// GitLab implements the Platform interface.
type GitLab struct{}

// NewGitLab creates a new GitLab client.
func NewGitLab(ctx context.Context, cfg *config.GitLab) (*GitLab, error) {
	return &GitLab{}, nil
}

// AssignReviewers adds users to the reviewers list of a Merge Request.
func (g *GitLab) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	return &AssignReviewersResult{}, nil
}

// GetLatestApprovers retrieves the reviewers whose latest review is an
// approval.
func (g *GitLab) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	return &GetLatestApproversResult{}, nil
}

// GetUserRepoPermissions returns a user's access level to a GitLab project.
func (g *GitLab) GetUserRepoPermissions(ctx context.Context) (string, error) {
	return "", nil
}

// GetUserTeamMemberships retrieves a list of groups that a user is a
// member of.
func (g *GitLab) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	return []string{}, nil
}

// GetPolicyData retrieves the required data for policy evaluation.
func (g *GitLab) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	return &GetPolicyDataResult{}, nil
}

// StoragePrefix generates the unique storage prefix for the GitLab platform type.
func (g *GitLab) StoragePrefix(ctx context.Context) (string, error) {
	return "", nil
}
