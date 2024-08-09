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
	"fmt"

	"github.com/sethvargo/go-githubactions"
)

// GitHubParams defines the required values sourced from the GitHub context.
type GitHubParams struct {
	Owner             string
	Repository        string
	PullRequestNumber int
}

// FromGitHubContext retrieves the required params from the GitHub context.
func (g *GitHubParams) FromGitHubContext(gctx *githubactions.GitHubContext) error {
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

	g.Owner = owner
	g.Repository = repo
	g.PullRequestNumber = pr

	return nil
}
