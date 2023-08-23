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

package workflows

import (
	"errors"
	"fmt"

	"github.com/sethvargo/go-githubactions"
)

// PlanStatusCommentsConfig defines the of configuration required for running the plan status comments
// command for GitHub workflows.
type PlanStatusCommentsConfig struct {
	// GitHub context values
	ServerURL  string // this value is used to generate URLs for creating links in pull request comments
	RunID      int64
	RunAttempt int64
}

// MapGitHubContext maps values from the GitHub context.
func (c *PlanStatusCommentsConfig) MapGitHubContext(context *githubactions.GitHubContext) error {
	var merr error

	c.ServerURL = context.ServerURL
	if c.ServerURL == "" {
		merr = errors.Join(merr, fmt.Errorf("GITHUB_SERVER_URL is required"))
	}

	c.RunID = context.RunID
	if c.RunID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("GITHUB_RUN_ID is required"))
	}

	c.RunAttempt = context.RunAttempt
	if c.RunAttempt <= 0 {
		merr = errors.Join(merr, fmt.Errorf("GITHUB_RUN_ATTEMPT is required"))
	}

	return merr
}

// ValidatePermissionsConfig defines the of configuration required for running the
// validate permissions command for GitHub workflows.
type ValidatePermissionsConfig struct {
	// GitHub context values
	Actor string
}

// MapGitHubContext maps values from the GitHub context.
func (c *ValidatePermissionsConfig) MapGitHubContext(context *githubactions.GitHubContext) error {
	var merr error

	c.Actor = context.Actor
	if c.Actor == "" {
		merr = errors.Join(merr, fmt.Errorf("GITHUB_ACTOR is required"))
	}

	return merr
}
