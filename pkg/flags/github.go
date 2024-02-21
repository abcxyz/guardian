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

import (
	"errors"
	"fmt"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/githubauth"
)

// GitHubFlags represent the shared GitHub flags among all commands.
// Embed this struct into any commands that interact with GitHub.
type GitHubFlags struct {
	FlagGitHubToken             string
	FlagGitHubOwner             string
	FlagGitHubRepo              string
	FlagGitHubAppID             string
	FlagGitHubAppInstallationID string
	FlagGitHubAppPrivateKeyPEM  string
}

func (g *GitHubFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("GITHUB OPTIONS")

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

	f.StringVar(&cli.StringVar{
		Name:   "github-app-id",
		EnvVar: "GITHUB_APP_ID",
		Target: &g.FlagGitHubAppID,
		Usage:  "The ID of GitHub App to use for requesting tokens to make GitHub API calls.",
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-installation-id",
		EnvVar: "GITHUB_APP_INSTALLATION_ID",
		Target: &g.FlagGitHubAppInstallationID,
		Usage:  "The Installation ID of GitHub App to use for requesting tokens to make GitHub API calls.",
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-private-key-pem",
		EnvVar: "GITHUB_APP_PRIVATE_KEY_PEM",
		Target: &g.FlagGitHubAppPrivateKeyPEM,
		Usage:  "The PEM formatted private key to use with the GitHub App.",
	})

	set.AfterParse(func(merr error) error {
		if g.FlagGitHubToken == "" && g.FlagGitHubAppID == "" {
			merr = errors.Join(merr, fmt.Errorf("one of github token or github app id are required"))
		}
		if g.FlagGitHubToken != "" && g.FlagGitHubAppID != "" {
			merr = errors.Join(merr, fmt.Errorf("only one of github token or github app id are allowed"))
		}
		if g.FlagGitHubAppID != "" && g.FlagGitHubAppInstallationID == "" {
			merr = errors.Join(merr, fmt.Errorf("a github app installation id is required when using a github app id"))
		}
		if g.FlagGitHubAppID != "" && g.FlagGitHubAppPrivateKeyPEM == "" {
			merr = errors.Join(merr, fmt.Errorf("a github app private key is required when using a github app id"))
		}
		return merr
	})
}

func (g *GitHubFlags) TokenSource(permissions map[string]string) (githubauth.TokenSource, error) {
	if g.FlagGitHubToken != "" {
		githubTokenSource, err := githubauth.NewStaticTokenSource(g.FlagGitHubToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create github static token source: %w", err)
		}
		return githubTokenSource, nil
	} else {
		app, err := githubauth.NewApp(
			g.FlagGitHubAppID,
			g.FlagGitHubAppInstallationID,
			g.FlagGitHubAppPrivateKeyPEM,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		return app.SelectedReposTokenSource(permissions, g.FlagGitHubRepo), nil
	}
}
