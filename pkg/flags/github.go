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
	"errors"
	"fmt"
	"hash/crc32"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/githubauth"
)

// GitHubFlags represent the shared GitHub flags among all commands.
// Embed this struct into any commands that interact with GitHub.
type GitHubFlags struct {
	FlagIsGitHubActions                 bool
	FlagGitHubToken                     string
	FlagGitHubOwner                     string
	FlagGitHubRepo                      string
	FlagGitHubAppID                     string
	FlagGitHubAppInstallationID         string
	FlagGitHubAppPrivateKeyResourceName string
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
		Name:   "github-app-private-key-resource-name",
		EnvVar: "GITHUB_APP_PRIVATE_KEY_RESOURCE_NAME",
		Target: &g.FlagGitHubAppPrivateKeyResourceName,
		Usage:  "The resource name of the private key to use with the GitHub App.",
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
		if g.FlagGitHubAppID != "" && g.FlagGitHubAppPrivateKeyResourceName == "" {
			merr = errors.Join(merr, fmt.Errorf("a github app private key resource name is required when using a github app id"))
		}
		return merr
	})
}

func (g *GitHubFlags) GetTokenSource(ctx context.Context, permissions map[string]string) (githubauth.TokenSource, error) {
	if g.FlagGitHubToken != "" {
		githubTokenSource, err := githubauth.NewStaticTokenSource(g.FlagGitHubToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create github static token source: %w", err)
		}
		return githubTokenSource, nil
	} else {
		sm, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create secret manager client: %w", err)
		}
		defer sm.Close()

		privateKeyPEM, err := accessSecret(ctx, sm, g.FlagGitHubAppPrivateKeyResourceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get github private key pem: %w", err)
		}

		app, err := githubauth.NewApp(g.FlagGitHubAppID, g.FlagGitHubAppInstallationID, privateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		return app.SelectedReposTokenSource(permissions, g.FlagGitHubRepo), nil
	}
}

// AccessSecret reads a secret from Secret Manager using the given client and
// validates that it was not corrupted during retrieval. The secretResourceName
// should be in the format: 'projects/*/secrets/*/versions/*'.
func accessSecret(ctx context.Context, client *secretmanager.Client, secretResourceName string) (string, error) {
	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretResourceName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", secretResourceName, err)
	}
	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(result.Payload.Data, crc32c))
	if checksum != *result.Payload.DataCrc32C {
		return "", fmt.Errorf("failed to access secret %s: data corrupted", secretResourceName)
	}
	return string(result.Payload.Data), nil
}
