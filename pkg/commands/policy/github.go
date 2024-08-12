// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

// GitHubParams defines the required values sourced from the GitHub context.
type GitHubParams struct {
	Owner             string
	Repository        string
	PullRequestNumber int
}

// FromGitHubContext retrieves the required params from the GitHub context.
func (p *GitHubParams) FromGitHubContext(gctx *githubactions.GitHubContext) error {
	owner, repo := gctx.Repo()
	if owner == "" {
		return fmt.Errorf("failed to get the repository owner")
	}
	if repo == "" {
		return fmt.Errorf("failed to get the repository name")
	}

	number, found := gctx.Event["number"]
	if !found {
		return fmt.Errorf("failed to get pull request number")
	}
	pr, ok := number.(int)
	if !ok {
		return fmt.Errorf("pull request number is not of type int")
	}

	p.Owner = owner
	p.Repository = repo
	p.PullRequestNumber = pr

	return nil
}

// GitHub implements the CodeReviewPlatform interface.
type GitHub struct {
	client github.GitHub
	params *GitHubParams
}

// NewGitHub creates a new GitHub wrapper for calling the GitHub API for policy
// enforcement. It implements the CodeReviewPlatform interface.
func NewGitHub(ctx context.Context, gitHubFlags *flags.GitHubFlags, actionOpts githubactions.Option) (*GitHub, error) {
	tokenSource, err := gitHubFlags.TokenSource(ctx, map[string]string{
		"contents":      "read",
		"pull_requests": "write",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	token, err := tokenSource.GitHubToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	var gitHubParams GitHubParams
	action := githubactions.New(actionOpts)
	actx, err := action.Context()
	if err != nil {
		return nil, fmt.Errorf("failed to load github context: %w", err)
	}
	if err = gitHubParams.FromGitHubContext(actx); err != nil {
		return nil, fmt.Errorf("failed to get github params from github context: %w", err)
	}

	client := github.NewClient(
		ctx,
		token,
	)

	return &GitHub{
		client: client,
		params: &gitHubParams,
	}, nil
}

// RequestReviewers calls the GitHub API to assign users and teams as reviewers
// for the current pull request. This makes a request per user and team to avoid
// an uncaught behavior from the GitHub API, which does not assign any of the
// provided reviewers if any of the principals exist on the pending review list.
func (g *GitHub) RequestReviewers(ctx context.Context, users, teams []string) error {
	logger := logging.FromContext(ctx)

	for _, u := range users {
		_, err := g.client.RequestReviewers(ctx, g.params.Owner, g.params.Repository, g.params.PullRequestNumber, []string{u}, nil)
		if err != nil {
			logger.ErrorContext(ctx, "failed to request review",
				"user", u,
				"error", err)
		}
	}
	for _, t := range teams {
		_, err := g.client.RequestReviewers(ctx, g.params.Owner, g.params.Repository, g.params.PullRequestNumber, nil, []string{t})
		if err != nil {
			logger.ErrorContext(ctx, "failed to request review",
				"team", t,
				"error", err)
		}
	}

	return nil
}
