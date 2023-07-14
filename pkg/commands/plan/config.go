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
	"fmt"

	"github.com/sethvargo/go-githubactions"
)

// Config defines the of configuration required for running the plan action.
type Config struct {
	// GitHub context values
	IsAction          bool
	EventName         string
	RepositoryOwner   string
	RepositoryName    string
	PullRequestNumber int
	ServerURL         string
	RunID             int64
	RunAttempt        int64
}

// MapGitHubContext maps values from the GitHub context to the PlanConfig.
func (c *Config) MapGitHubContext(context *githubactions.GitHubContext) error {
	repoOwner, repoName := context.Repo()

	c.IsAction = context.Actions
	c.EventName = context.EventName
	c.RepositoryOwner = repoOwner
	c.RepositoryName = repoName
	c.ServerURL = context.ServerURL
	c.RunID = context.RunID
	c.RunAttempt = context.RunAttempt

	if _, ok := context.Event["number"]; !ok {
		return fmt.Errorf("failed to get pull request number from github event")
	}

	// parsing a json file into this map causes javascript numbers to be float64
	v, ok := context.Event["number"].(float64)
	if !ok {
		return fmt.Errorf("failed to get pull request number, github.event.number is not a number")
	}

	c.PullRequestNumber = int(v)

	return nil
}
