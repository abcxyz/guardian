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
	"context"
	"fmt"

	"github.com/sethvargo/go-githubactions"

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
	FlagGitHubServerURL         string
	FlagGitHubRunID             int64
	FlagGitHubRunAttempt        int64
	FlagGitHubJob               string
	FlagGitHubPullRequestNumber int
	FlagGitHubSHA               string
}

func (g *GitHubFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("GITHUB OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		EnvVar: "GITHUB_TOKEN",
		Target: &g.FlagGitHubToken,
		Usage:  "The GitHub access token to make GitHub API calls. This value is automatically set on GitHub Actions.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-owner",
		Target:  &g.FlagGitHubOwner,
		Example: "organization-name",
		Usage:   "The GitHub repository owner.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-repo",
		Target:  &g.FlagGitHubRepo,
		Example: "repository-name",
		Usage:   "The GitHub repository name.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-id",
		EnvVar: "GITHUB_APP_ID",
		Target: &g.FlagGitHubAppID,
		Usage:  "The ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-installation-id",
		EnvVar: "GITHUB_APP_INSTALLATION_ID",
		Target: &g.FlagGitHubAppInstallationID,
		Usage:  "The Installation ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-private-key-pem",
		EnvVar: "GITHUB_APP_PRIVATE_KEY_PEM",
		Target: &g.FlagGitHubAppPrivateKeyPEM,
		Usage:  "The PEM formatted private key to use with the GitHub App.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-server-url",
		EnvVar: "GITHUB_SERVER_URL",
		Target: &g.FlagGitHubServerURL,
		Usage:  "The GitHub server URL.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-id",
		EnvVar: "GITHUB_RUN_ID",
		Target: &g.FlagGitHubRunID,
		Usage:  "The GitHub workflow run ID.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-attempt",
		EnvVar: "GITHUB_RUN_ATTEMPT",
		Target: &g.FlagGitHubRunAttempt,
		Usage:  "The GitHub workflow run attempt.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-job",
		EnvVar: "GITHUB_JOB",
		Target: &g.FlagGitHubJob,
		Usage:  "The GitHub job id.",
		Hidden: true,
	})

	f.IntVar(&cli.IntVar{
		Name:   "github-pull-request-number",
		Target: &g.FlagGitHubPullRequestNumber,
		Usage:  "The GitHub pull request number.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-commit-sha",
		EnvVar: "GITHUB_SHA",
		Target: &g.FlagGitHubSHA,
		Usage:  "The GitHub SHA.",
		Hidden: true,
	})
}

// FromGitHubContext maps missing GitHub flag values from the GitHub context.
func (g *GitHubFlags) FromGitHubContext(gctx *githubactions.GitHubContext) {
	owner, repo := gctx.Repo()

	if g.FlagGitHubOwner == "" {
		g.FlagGitHubOwner = owner
	}

	if g.FlagGitHubRepo == "" {
		g.FlagGitHubRepo = repo
	}

	eventNumber := gctx.Event["number"]
	if g.FlagGitHubPullRequestNumber <= 0 && eventNumber != nil {
		prNumber, ok := eventNumber.(int)
		if ok {
			g.FlagGitHubPullRequestNumber = prNumber
		}
	}
}

// TokenSource creates a token source for a github client to call the GitHub API.
func (g *GitHubFlags) TokenSource(ctx context.Context, permissions map[string]string) (githubauth.TokenSource, error) {
	if g.FlagGitHubToken != "" {
		githubTokenSource, err := githubauth.NewStaticTokenSource(g.FlagGitHubToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create github static token source: %w", err)
		}
		return githubTokenSource, nil
	} else {
		app, err := githubauth.NewApp(
			g.FlagGitHubAppID,
			g.FlagGitHubAppPrivateKeyPEM,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, g.FlagGitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		return installation.SelectedReposTokenSource(permissions, g.FlagGitHubRepo), nil
	}
}
