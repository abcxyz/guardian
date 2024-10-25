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

package reporter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v53/github"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

var _ Reporter = (*GitHubReporter)(nil)

const (
	githubMaxCommentLength = 65536
)

// GitHubReporter implements the reporter interface for writing GitHub comments.
type GitHubReporter struct {
	gitHubClient github.GitHub
	inputs       *GitHubReporterInputs
	logURL       string
}

// GitHubReporterInputs are the inputs used by the GitHub reporter.
type GitHubReporterInputs struct {
	GitHubOwner             string
	GitHubRepo              string
	GitHubServerURL         string
	GitHubRunID             int64
	GitHubRunAttempt        int64
	GitHubJob               string
	GitHubJobName           string
	GitHubPullRequestNumber int
	GitHubSHA               string
}

// Validate validates the required inputs.
func (i *GitHubReporterInputs) Validate() error {
	var merr error
	if i.GitHubOwner == "" {
		merr = errors.Join(merr, fmt.Errorf("github owner is required"))
	}

	if i.GitHubRepo == "" {
		merr = errors.Join(merr, fmt.Errorf("github repo is required"))
	}

	if i.GitHubPullRequestNumber <= 0 && i.GitHubSHA == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github pull request number or github sha are required"))
	}

	return merr
}

// NewGitHubReporter creates a new GitHubReporter.
func NewGitHubReporter(ctx context.Context, gc github.GitHub, i *GitHubReporterInputs) (Reporter, error) {
	logger := logging.FromContext(ctx)

	if gc == nil {
		return nil, fmt.Errorf("github client is required")
	}

	if err := i.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate github reporter inputs: %w", err)
	}

	if i.GitHubPullRequestNumber <= 0 {
		prResponse, err := gc.ListPullRequestsForCommit(ctx, i.GitHubOwner, i.GitHubRepo, i.GitHubSHA, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get pull request number for commit sha: %w", err)
		}

		if len(prResponse.PullRequests) == 0 {
			return nil, fmt.Errorf("no pull requests found for commit sha: %s", i.GitHubSHA)
		}

		i.GitHubPullRequestNumber = prResponse.PullRequests[0].Number
	}

	logger.DebugContext(ctx, "computed pull request number", "computed_pull_request_number", i.GitHubPullRequestNumber)

	r := &GitHubReporter{
		gitHubClient: gc,
		inputs:       i,
	}

	var logURL string
	if i.GitHubServerURL != "" || i.GitHubRunID > 0 || i.GitHubRunAttempt > 0 {
		logURL = fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", i.GitHubServerURL, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID, i.GitHubRunAttempt)
	}

	if i.GitHubJobName != "" {
		resolvedURL, err := gc.ResolveJobLogsURL(ctx, i.GitHubJobName, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID)
		if err != nil {
			resolvedURL = logURL
		}
		r.logURL = resolvedURL
	}

	return r, nil
}

// Status implements the reporter Status function by writing a GitHub comment.
func (g *GitHubReporter) Status(ctx context.Context, st Status, p *StatusParams) error {
	msg, err := statusMessage(st, p, g.logURL, githubMaxCommentLength)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	_, err = g.gitHubClient.CreateIssueComment(
		ctx,
		g.inputs.GitHubOwner,
		g.inputs.GitHubRepo,
		g.inputs.GitHubPullRequestNumber,
		msg.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// EntrypointsSummary implements the reporter EntrypointsSummary function by writing a GitHub comment.
func (g *GitHubReporter) EntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	msg, err := entrypointsSummaryMessage(p, g.logURL)
	if err != nil {
		return fmt.Errorf("failed to generate summary message: %w", err)
	}

	if _, err = g.gitHubClient.CreateIssueComment(
		ctx,
		g.inputs.GitHubOwner,
		g.inputs.GitHubRepo,
		g.inputs.GitHubPullRequestNumber,
		msg.String(),
	); err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// Clear deletes all github comments with the guardian prefix.
func (g *GitHubReporter) Clear(ctx context.Context) error {
	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		response, err := g.gitHubClient.ListIssueComments(ctx, g.inputs.GitHubOwner, g.inputs.GitHubRepo, g.inputs.GitHubPullRequestNumber, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Comments == nil {
			return nil
		}

		for _, comment := range response.Comments {
			// prefix is not found, skip
			if !strings.HasPrefix(comment.Body, commentPrefix) {
				continue
			}

			// found the prefix, delete the comment
			if err := g.gitHubClient.DeleteIssueComment(ctx, g.inputs.GitHubOwner, g.inputs.GitHubRepo, comment.ID); err != nil {
				return fmt.Errorf("failed to delete comment: %w", err)
			}
		}

		if response.Pagination == nil {
			return nil
		}
		listOpts.Page = response.Pagination.NextPage
	}
}
