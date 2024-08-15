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

package github

import (
	"time"

	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/pkg/cli"
)

// Config is the config values for the GitHub client.
type Config struct {
	// Retry
	MaxRetries        uint64
	InitialRetryDelay time.Duration
	MaxRetryDelay     time.Duration

	// Auth
	GitHubToken             string
	GitHubOwner             string
	GitHubRepo              string
	GitHubAppID             string
	GitHubAppInstallationID string
	GitHubAppPrivateKeyPEM  string
	Permissions             map[string]string

	GitHubServerURL         string
	GitHubRunID             int64
	GitHubRunAttempt        int64
	GitHubJob               string
	GitHubPullRequestNumber int
	GitHubSHA               string
}

func (c *Config) RegisterFlags(set *cli.FlagSet) {
	githubContext, _ := githubactions.New().Context()
	owner, repo := githubContext.Repo()
	prNumber, _ := githubContext.Event["number"].(int)

	f := set.NewSection("GITHUB OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		EnvVar: "GITHUB_TOKEN",
		Target: &c.GitHubToken,
		Usage:  "The GitHub access token to make GitHub API calls. This value is automatically set on GitHub Actions.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-owner",
		Target:  &c.GitHubOwner,
		Default: owner,
		Example: "organization-name",
		Usage:   "The GitHub repository owner.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-repo",
		Target:  &c.GitHubRepo,
		Default: repo,
		Example: "repository-name",
		Usage:   "The GitHub repository name.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-id",
		EnvVar: "GITHUB_APP_ID",
		Target: &c.GitHubAppID,
		Usage:  "The ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-installation-id",
		EnvVar: "GITHUB_APP_INSTALLATION_ID",
		Target: &c.GitHubAppInstallationID,
		Usage:  "The Installation ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-private-key-pem",
		EnvVar: "GITHUB_APP_PRIVATE_KEY_PEM",
		Target: &c.GitHubAppPrivateKeyPEM,
		Usage:  "The PEM formatted private key to use with the GitHub App.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-server-url",
		EnvVar: "GITHUB_SERVER_URL",
		Target: &c.GitHubServerURL,
		Usage:  "The GitHub server URL.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-id",
		EnvVar: "GITHUB_RUN_ID",
		Target: &c.GitHubRunID,
		Usage:  "The GitHub workflow run ID.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-attempt",
		EnvVar: "GITHUB_RUN_ATTEMPT",
		Target: &c.GitHubRunAttempt,
		Usage:  "The GitHub workflow run attempt.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-job",
		EnvVar: "GITHUB_JOB",
		Target: &c.GitHubJob,
		Usage:  "The GitHub job id.",
		Hidden: true,
	})

	f.IntVar(&cli.IntVar{
		Name:    "github-pull-request-number",
		Target:  &c.GitHubPullRequestNumber,
		Default: prNumber,
		Usage:   "The GitHub pull request number.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-commit-sha",
		EnvVar: "GITHUB_SHA",
		Target: &c.GitHubSHA,
		Usage:  "The GitHub SHA.",
		Hidden: true,
	})
}
