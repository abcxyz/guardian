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
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-githubactions"
	"golang.org/x/oauth2"

	"github.com/abcxyz/pkg/cli"
)

// Config is the config values for the GitHub client.
type Config struct {
	MaxRetries        uint64
	InitialRetryDelay time.Duration
	MaxRetryDelay     time.Duration

	Token             string
	Owner             string
	Repo              string
	AppID             string
	AppInstallationID string
	AppPrivateKeyPEM  string
	ServerURL         string
	RunID             int64
	RunAttempt        int64
	Job               string
	PullRequestNumber int
	SHA               string
}

func (c *Config) Register(set *cli.FlagSet) {
	f := set.NewSection("GITHUB OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		EnvVar: "GITHUB_TOKEN",
		Target: &c.Token,
		Usage:  "The GitHub access token to make GitHub API calls. This value is automatically set on GitHub Actions.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-owner",
		Target:  &c.Owner,
		Example: "organization-name",
		Usage:   "The GitHub repository owner.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-repo",
		Target:  &c.Repo,
		Example: "repository-name",
		Usage:   "The GitHub repository name.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-id",
		EnvVar: "GITHUB_APP_ID",
		Target: &c.AppID,
		Usage:  "The ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-installation-id",
		EnvVar: "GITHUB_APP_INSTALLATION_ID",
		Target: &c.AppInstallationID,
		Usage:  "The Installation ID of GitHub App to use for requesting tokens to make GitHub API calls.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-app-private-key-pem",
		EnvVar: "GITHUB_APP_PRIVATE_KEY_PEM",
		Target: &c.AppPrivateKeyPEM,
		Usage:  "The PEM formatted private key to use with the GitHub App.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-server-url",
		EnvVar: "GITHUB_SERVER_URL",
		Target: &c.ServerURL,
		Usage:  "The GitHub server URL.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-id",
		EnvVar: "GITHUB_RUN_ID",
		Target: &c.RunID,
		Usage:  "The GitHub workflow run ID.",
		Hidden: true,
	})

	f.Int64Var(&cli.Int64Var{
		Name:   "github-run-attempt",
		EnvVar: "GITHUB_RUN_ATTEMPT",
		Target: &c.RunAttempt,
		Usage:  "The GitHub workflow run attempt.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-job",
		EnvVar: "GITHUB_JOB",
		Target: &c.Job,
		Usage:  "The GitHub job id.",
		Hidden: true,
	})

	f.IntVar(&cli.IntVar{
		Name:   "github-pull-request-number",
		Target: &c.PullRequestNumber,
		Usage:  "The GitHub pull request number.",
		Hidden: true,
	})

	f.StringVar(&cli.StringVar{
		Name:   "github-commit-sha",
		EnvVar: "GITHUB_SHA",
		Target: &c.SHA,
		Usage:  "The GitHub SHA.",
		Hidden: true,
	})

	set.AfterParse(func(existingErr error) error {
		ghCtx, err := githubactions.New().Context()
		if err != nil {
			return fmt.Errorf("failed to create github context: %w", err)
		}

		return c.afterParse(ghCtx)
	})
}

// afterParse maps missing GitHub config values from the GitHub context.
func (c *Config) afterParse(ghCtx *githubactions.GitHubContext) error {
	owner, repo := ghCtx.Repo()

	if c.Owner == "" {
		c.Owner = owner
	}

	if c.Repo == "" {
		c.Repo = repo
	}

	eventNumber := ghCtx.Event["number"]
	if c.PullRequestNumber <= 0 && eventNumber != nil {
		prNumber, ok := eventNumber.(int)
		if ok {
			c.PullRequestNumber = prNumber
		}
	}

	return nil
}

// NewGitHubClient creates a new GitHub client from the GitHub flags.
func (c *Config) NewGitHubClient(ctx context.Context, perms map[string]string) (*GitHubClient, error) {
	ts, err := c.TokenSource(ctx, perms)
	if err != nil {
		return nil, fmt.Errorf("failed to create github token source: %w", err)
	}
	return NewClient(ctx, ts), nil
}

// TokenSource creates a token source for a github client to call the GitHub API.
func (c *Config) TokenSource(ctx context.Context, permissions map[string]string) (oauth2.TokenSource, error) {
	return TokenSource(ctx, &TokenSourceInputs{
		GitHubToken:             c.Token,
		GitHubAppID:             c.AppID,
		GitHubAppPrivateKeyPEM:  c.AppPrivateKeyPEM,
		GitHubAppInstallationID: c.AppInstallationID,
		GitHubRepo:              c.Repo,
		Permissions:             permissions,
	})
}
