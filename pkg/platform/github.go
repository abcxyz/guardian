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
	"fmt"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

var _ Platform = (*GitHub)(nil)

// GitHub implements the Platform interface.
type GitHub struct {
	cfg    *github.Config
	client github.GitHub
}

// NewGitHub creates a new GitHub client.
func NewGitHub(ctx context.Context, cfg *github.Config) (*GitHub, error) {
	client, err := github.NewGitHubClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitHub client: %w", err)
	}

	return &GitHub{
		cfg:    cfg,
		client: client,
	}, nil
}

// AssignReviewers assigns a list of users and teams as reviewers of a target
// Pull Request. The implementation assigns one principal per request because
// mixing existing reviewers in pending review state with new reviewers will
// result in a no-op without errors thrown.
func (g *GitHub) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	logger := logging.FromContext(ctx)
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}

	var result AssignReviewersResult
	for _, u := range inputs.Users {
		if _, err := g.client.RequestReviewers(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber, []string{u}, nil); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"user", u,
				"error", err,
			)
			continue
		}
		result.Users = append(result.Users, u)
	}
	for _, t := range inputs.Teams {
		if _, err := g.client.RequestReviewers(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber, nil, []string{t}); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"team", t,
				"error", err,
			)
			continue
		}
		result.Teams = append(result.Teams, t)
	}

	if len(result.Users) == 0 && len(result.Teams) == 0 {
		return nil, fmt.Errorf("failed to assign all requested reviewers to pull request")
	}

	return &result, nil
}
