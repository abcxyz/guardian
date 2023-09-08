// Copyright 2023 Google LLC
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

package drift

import (
	"context"
	"fmt"

	"github.com/abcxyz/guardian/pkg/github"
	githubAPI "github.com/google/go-github/v53/github"
)

type GitHubDriftIssueService struct {
	gh         github.GitHub
	owner      string
	repo       string
	issueTitle string
	issueBody  string
}

func NewGitHubDriftIssueService(gh github.GitHub, owner, repo, issueTitle, issueBody string) *GitHubDriftIssueService {
	return &GitHubDriftIssueService{gh, owner, repo, issueTitle, issueBody}
}

func (s *GitHubDriftIssueService) CreateOrUpdateIssue(ctx context.Context, assignees, labels []string, message string) error {
	// Labels are used to uniquely identify Drift issues.
	if len(labels) == 0 {
		return fmt.Errorf("invalid argument - at least one 'label' must be provided")
	}

	issues, err := s.gh.ListIssues(ctx, s.owner, s.repo, &githubAPI.IssueListByRepoOptions{
		Labels: labels,
		State:  github.Open,
	})
	if err != nil {
		return fmt.Errorf("failed to list GitHub issues for %s/%s: %w", s.owner, s.repo, err)
	}

	var issueNumber int
	if len(issues) == 0 {
		issue, err := s.gh.CreateIssue(ctx, s.owner, s.repo, issueTitle, issueBody, assignees, labels)
		if err != nil {
			return fmt.Errorf("failed to create GitHub issue for %s/%s with assignees %s and labels %s: %w", s.owner, s.repo, assignees, labels, err)
		}
		issueNumber = issue.Number
	} else {
		issueNumber = issues[0].Number
	}

	if _, err := s.gh.CreateIssueComment(ctx, s.owner, s.repo, issueNumber, message); err != nil {
		return fmt.Errorf("failed to comment on issue %s/%s %d: %w", s.owner, s.repo, issueNumber, err)
	}

	return nil
}

func (s *GitHubDriftIssueService) CloseIssues(ctx context.Context, labels []string) error {
	// Labels are used to uniquely identify Drift issues.
	if len(labels) == 0 {
		return fmt.Errorf("invalid argument - at least one 'label' must be provided")
	}
	issues, err := s.gh.ListIssues(ctx, s.owner, s.repo, &githubAPI.IssueListByRepoOptions{
		Labels: labels,
		State:  github.Open,
	})
	if err != nil {
		return fmt.Errorf("failed to list GitHub issues for %s/%s: %w", s.owner, s.repo, err)
	}
	for _, issueToClose := range issues {
		if _, err := s.gh.CreateIssueComment(ctx, s.owner, s.repo, issueToClose.Number, "Drift Resolved."); err != nil {
			return fmt.Errorf("failed to comment on issue %s/%s %d: %w", s.owner, s.repo, issueToClose.Number, err)
		}
		if err := s.gh.CloseIssue(ctx, s.owner, s.repo, issueToClose.Number); err != nil {
			return fmt.Errorf("failed to close GitHub issue for %s/%s %d: %w", s.owner, s.repo, issueToClose.Number, err)
		}
	}
	return nil
}
