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

package plan

import (
	"github.com/sethvargo/go-githubactions"
)

// Config defines the of configuration required for running the plan action.
type Config struct {
	// GitHub context values
	ServerURL  string
	RunID      int64
	RunAttempt int64
}

// MapGitHubContext maps values from the GitHub context to the PlanConfig.
func (c *Config) MapGitHubContext(context *githubactions.GitHubContext) error {
	c.ServerURL = context.ServerURL
	c.RunID = context.RunID
	c.RunAttempt = context.RunAttempt
	return nil
}
