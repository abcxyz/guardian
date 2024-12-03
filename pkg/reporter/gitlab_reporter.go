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
	"fmt"

	gitlab "github.com/xanzy/go-gitlab"
)

const gitLabMaxCommentLength = 1000000

var _ Reporter = (*GitLabReporter)(nil)

// GitLabReporter implements the Reporter interface.
type GitLabReporter struct {
	gitLabClient *gitlab.Client
	inputs       *GitLabReporterInputs
	logURL       string
}

// GitLabReporterInputs are the inputs used by the GitLab reporter.
type GitLabReporterInputs struct {
	GitLabProjectID      int
	GitLabMergeRequestID int
}

func (i *GitLabReporterInputs) Validate() error {
	if i.GitLabProjectID == 0 {
		return fmt.Errorf("gitlab project id is required")
	}
	if i.GitLabMergeRequestID == 0 {
		return fmt.Errorf("gitlab merge request id is required")
	}
	return nil
}

func NewGitLabReporter(ctx context.Context, gc *gitlab.Client, i *GitLabReporterInputs) (*GitLabReporter, error) {
	if gc == nil {
		return nil, fmt.Errorf("gitlab client is required")
	}

	if err := i.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate gitlab reporter inputs: %w", err)
	}

	return &GitLabReporter{
		gitLabClient: gc,
		inputs:       i,
	}, nil
}

// Status reports the status of a run.
func (g *GitLabReporter) Status(ctx context.Context, st Status, p *StatusParams) error {
	msg, err := statusMessage(st, p, g.logURL, gitLabMaxCommentLength)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	b := msg.String()
	_, _, err = g.gitLabClient.Notes.CreateMergeRequestNote(g.inputs.GitLabProjectID, g.inputs.GitLabMergeRequestID, &gitlab.CreateMergeRequestNoteOptions{
		Body: &b,
	})
	if err != nil {
		return fmt.Errorf("failed to create merge request note: %w", err)
	}

	return nil
}

// EntrypointsSummary reports the summary for the entrypionts command.
func (g *GitLabReporter) EntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error {
	return nil
}

// Clear clears any existing reports that can be removed.
func (g *GitLabReporter) Clear(ctx context.Context) error {
	return nil
}
