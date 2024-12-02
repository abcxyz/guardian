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

import "context"

var _ Reporter = (*GitLabReporter)(nil)

// GitLabReporter implements the Reporter interface.
type GitLabReporter struct{}

// Status reports the status of a run.
func (g *GitLabReporter) Status(ctx context.Context, status Status, params *StatusParams) error {
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
