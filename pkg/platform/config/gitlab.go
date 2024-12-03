// Copyright 2024 The Authors (see AUTHORS file)
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

// Package config defines configuration options for each supported platform.
package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"

	"github.com/abcxyz/pkg/cli"
)

// GitLab is the configurable values for the GitLab client.
type GitLab struct {
	GitLabBaseURL        string
	GitLabToken          string
	GitLabMergeRequestID int
	GitLabProjectID      int
}

// GitLabPredefined is the expected variables available in GitLab Pipelines.
// See https://docs.gitlab.com/ee/ci/variables/predefined_variables.html.
type GitLabPredefined struct {
	Host           string `env:"CI_SERVER_HOST"`
	Token          string `env:"CI_JOB_TOKEN"`
	ProjectID      int    `env:"CI_PROJECT_ID"`
	MergeRequestID int    `env:"CI_MERGE_REQUEST_ID"`
}

func (g *GitLab) RegisterFlags(set *cli.FlagSet) {
	f := set.NewSection("GITLAB OPTIONS")

	configDefault := &GitLabPredefined{}
	_ = envconfig.Process(context.Background(), configDefault)

	f.StringVar(&cli.StringVar{
		Name:    "gitlab-base-url",
		EnvVar:  "GITLAB_BASE_URL",
		Target:  &g.GitLabBaseURL,
		Example: "https://gitlab.example.domain.com/api/v4",
		Default: fmt.Sprintf("%s/api/v4", configDefault.Host),
		Usage:   "The base URL of the GitLab instance.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "gitlab-token",
		EnvVar:  "GITLAB_TOKEN",
		Target:  &g.GitLabToken,
		Default: configDefault.Token,
		Usage:   "The GitLab access token to make GitLab API calls.",
		Hidden:  true,
	})

	f.IntVar(&cli.IntVar{
		Name:    "gitlab-project-id",
		EnvVar:  "GITLAB_PROJECT_ID",
		Target:  &g.GitLabProjectID,
		Default: configDefault.ProjectID,
		Usage:   "The numeric ID of the GitLab project",
		Hidden:  true,
	})

	f.IntVar(&cli.IntVar{
		Name:    "gitlab-merge-request-id",
		Target:  &g.GitLabMergeRequestID,
		Example: "123",
		Default: configDefault.MergeRequestID,
		Usage:   "The numeric ID of the GitLab merge request",
		Hidden:  true,
	})
}
