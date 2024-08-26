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

// Package reporter provides an SDK for reporting Guardian results.
package reporter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
)

const (
	TypeNone   string = "none"
	TypeGitHub string = "github"
)

// SortedReporterTypes are the sorted Reporter types for printing messages and prediction.
var SortedReporterTypes = func() []string {
	allowed := append([]string{}, TypeNone, TypeGitHub)
	sort.Strings(allowed)
	return allowed
}()

// Status is the result of the operation Guardian is performing.
type Status string

// the supported statuses for reporters.
const (
	StatusSuccess     Status = Status("SUCCESS")    //nolint:errname // Not an error
	StatusFailure     Status = Status("FAILURE")    //nolint:errname // Not an error
	StatusNoOperation Status = Status("NO CHANGES") //nolint:errname // Not an error
	StatusUnknown     Status = Status("UNKNOWN")    //nolint:errname // Not an error
)

// StatusParams are the parameters for writing status reports.
type StatusParams struct {
	HasDiff   bool
	Details   string
	Dir       string
	Message   string
	Operation string
}

// EntrypointsSummaryParams are the parameters for writing entrypoints summary reports.
type EntrypointsSummaryParams struct {
	Message       string
	UpdateDirs    []string
	DestroyDirs   []string
	AbandonedDirs []string
}

// Reporter defines the minimum interface for a reporter.
type Reporter interface {
	// Status reports the status of a run.
	Status(ctx context.Context, status Status, params *StatusParams) error

	// EntrypointsSummary reports the summary for the entrypionts command.
	EntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error

	// Clear clears any existing reports that can be removed.
	Clear(ctx context.Context) error
}

// Config is the configuration needed to generate different reporter types.
type Config struct {
	GitHub github.Config
}

// NewReporter creates a new reporter based on the provided type.
func NewReporter(ctx context.Context, t string, c *Config) (Reporter, error) {
	if strings.EqualFold(t, TypeNone) {
		return NewNoopReporter(ctx)
	}

	if strings.EqualFold(t, TypeGitHub) {
		c.GitHub.Permissions = map[string]string{
			"contents":      "read",
			"pull_requests": "write",
		}

		gc, err := github.NewGitHubClient(ctx, &c.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}

		return NewGitHubReporter(ctx, gc, &GitHubReporterInputs{
			GitHubOwner:             c.GitHub.GitHubOwner,
			GitHubRepo:              c.GitHub.GitHubRepo,
			GitHubPullRequestNumber: c.GitHub.GitHubPullRequestNumber,
			GitHubServerURL:         c.GitHub.GitHubServerURL,
			GitHubRunID:             c.GitHub.GitHubRunID,
			GitHubRunAttempt:        c.GitHub.GitHubRunAttempt,
			GitHubJob:               c.GitHub.GitHubJob,
			GitHubJobName:           c.GitHub.GitHubJobName,
			GitHubSHA:               c.GitHub.GitHubSHA,
		})
	}

	return nil, fmt.Errorf("unknown reporter type: %s", t)
}
