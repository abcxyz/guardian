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
	"io"
	"sort"
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
)

const (
	TypeLocal  string = "local"
	TypeGitHub string = "github"
)

// SortedReporterTypes are the sorted Reporter types for printing messages and prediction.
var SortedReporterTypes = func() []string {
	allowed := append([]string{}, TypeLocal, TypeGitHub)
	sort.Strings(allowed)
	return allowed
}()

// Status is the result of the operation Guardian is performing.
type Status string

// the supported statuses for reporters.
const (
	StatusSuccess     Status = Status("SUCCESS")
	StatusFailure     Status = Status("FAILURE")
	StatusNoOperation Status = Status("NO CHANGES")
	StatusUnknown     Status = Status("UNKNOWN")
)

// Params are the parameters for writing reports.
type Params struct {
	HasDiff   bool
	Details   string
	Dir       string
	IsDestroy bool
	Message   string
	Operation string
}

// Reporter defines the minimum interface for a reporter.
type Reporter interface {
	// CreateStatus reports the status of a run.
	CreateStatus(ctx context.Context, status Status, params *Params) error

	// ClearStatus clears any existing statuses that can be removed.
	ClearStatus(ctx context.Context) error
}

// Config is the configuration needed to generate different reporter types.
type Config struct {
	GitHub github.Config
}

// NewReporter creates a new reporter based on the provided type.
func NewReporter(ctx context.Context, t string, c *Config, stdout io.Writer) (Reporter, error) {
	if strings.EqualFold(t, TypeLocal) {
		return NewLocalReporter(ctx, stdout)
	}

	c.GitHub.Permissions = map[string]string{
		"contents":      "read",
		"pull_requests": "write",
	}

	if strings.EqualFold(t, TypeGitHub) {
		gc, err := github.NewGitHubClient(ctx, &c.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}

		return NewGitHubReporter(ctx, gc, &GitHubReporterInputs{
			GitHubOwner:             c.GitHub.GitHubOwner,
			GitHubRepo:              c.GitHub.GitHubRepo,
			GitHubPullRequestNumber: c.GitHub.GitHubPullRequestNumber,
		})
	}

	return nil, fmt.Errorf("unknown reporter type: %s", t)
}
