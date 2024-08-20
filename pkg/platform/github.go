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

package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"

	gh "github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/githubauth"
)

var _ Platform = (*GitHub)(nil)

// GitHub implements the Platform interface.
type GitHub struct {
	cfg    *gh.Config
	client *github.Client
}

// NewGitHub creates a new GitHub client.
func NewGitHub(ctx context.Context, cfg *gh.Config) (*GitHub, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialRetryDelay <= 0 {
		cfg.InitialRetryDelay = 1 * time.Second
	}
	if cfg.MaxRetryDelay <= 0 {
		cfg.MaxRetryDelay = 20 * time.Second
	}

	var ts oauth2.TokenSource
	if cfg.GitHubToken != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: cfg.GitHubToken,
		})
	} else {
		app, err := githubauth.NewApp(cfg.GitHubAppID, cfg.GitHubAppPrivateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, cfg.GitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		ts = installation.SelectedReposOAuth2TokenSource(ctx, cfg.Permissions, cfg.GitHubRepo)
	}

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	g := &GitHub{
		cfg:    cfg,
		client: client,
	}

	return g, nil
}
