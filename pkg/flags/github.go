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

package flags

import "github.com/abcxyz/pkg/cli"

// GitHubFlags represent the shared GitHub flags among all commands.
// Embed this struct into any commands that interact with GitHub.
type GitHubFlags struct {
	FlagIsGitHubActions bool
	FlagGitHubToken     string
	FlagGitHubOwner     string
	FlagGitHubRepo      string
}

func (g *GitHubFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("GITHUB OPTIONS")

	f.BoolVar(&cli.BoolVar{
		Name:    "github-actions",
		EnvVar:  "GITHUB_ACTIONS",
		Target:  &g.FlagIsGitHubActions,
		Default: false,
		Usage:   "Is this running as a GitHub action.",
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		EnvVar: "GITHUB_TOKEN",
		Target: &g.FlagGitHubToken,
		Usage:  "The GitHub access token to make GitHub API calls. This value is automatically set on GitHub Actions.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-owner",
		Target:  &g.FlagGitHubOwner,
		Example: "organization-name",
		Usage:   "The GitHub repository owner.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-repo",
		Target:  &g.FlagGitHubRepo,
		Example: "repository-name",
		Usage:   "The GitHub repository name.",
	})
}
