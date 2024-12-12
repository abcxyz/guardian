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
	"errors"
	"fmt"
	"os"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var _ Platform = (*GitLab)(nil)

// Based on https://docs.gitlab.com/ee/administration/instance_limits.html#size-of-comments-and-descriptions-of-issues-merge-requests-and-epics.
const gitlabMaxCommentLength = 1000000

// TODO(gjonathanhong): Implement GitLab platform.

// GitLab implements the Platform interface.
type GitLab struct {
	cfg    *gitLabConfig
	client *gitlab.Client

	logURL string
}

type gitLabConfig struct {
	GitLabToken   string
	GitLabBaseURL string

	GitLabProjectID      int
	GitLabMergeRequestID int
}

type gitLabPredefinedConfig struct {
	CIJobToken       string
	CIServerHost     string
	CIProjectID      int
	CIMergeRequestID int
}

// Load retrieves the predefined GitLab CI/CD variables from environment. See
// https://docs.gitlab.com/ee/ci/variables/predefined_variables.html#predefined-variables.
func (c *gitLabPredefinedConfig) Load() {
	if v := os.Getenv("CI_JOB_TOKEN"); v != "" {
		c.CIJobToken = v
	}

	if v := os.Getenv("CI_API_V4_URL"); v != "" {
		c.CIServerHost = v
	}

	if v, err := strconv.Atoi(os.Getenv("CI_PROJECT_ID")); err == nil {
		c.CIProjectID = v
	}

	if v, err := strconv.Atoi(os.Getenv("CI_MERGE_REQUEST_ID")); err == nil {
		c.CIMergeRequestID = v
	}
}

func (c *gitLabConfig) RegisterFlags(set *cli.FlagSet) {
	f := set.NewSection("GITLAB OPTIONS")

	cfgDefaults := &gitLabPredefinedConfig{}
	cfgDefaults.Load()

	f.StringVar(&cli.StringVar{
		Name:    "gitlab-token",
		EnvVar:  "GITLAB_TOKEN",
		Target:  &c.GitLabToken,
		Default: cfgDefaults.CIJobToken,
		Usage:   "The GitLab access token to make GitLab API calls.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "gitlab-base-url",
		EnvVar:  "GITHUB_BASE_URL",
		Target:  &c.GitLabBaseURL,
		Example: "https://git.mydomain.com/api/v4",
		Default: cfgDefaults.CIServerHost,
		Usage:   "The base URL of the GitLab instance API.",
		Hidden:  true,
	})

	f.IntVar(&cli.IntVar{
		Name:    "gitlab-project-id",
		EnvVar:  "GITLAB_PROJECT_ID",
		Target:  &c.GitLabProjectID,
		Default: cfgDefaults.CIProjectID,
		Usage:   "The GitLab project ID.",
		Hidden:  true,
	})

	f.IntVar(&cli.IntVar{
		Name:    "gitlab-merge-request-id",
		EnvVar:  "GITLAB_MERGE_REQUEST_ID",
		Target:  &c.GitLabMergeRequestID,
		Default: cfgDefaults.CIMergeRequestID,
		Usage:   "The GitLab merge request ID.",
		Hidden:  true,
	})
}

// NewGitLab creates a new GitLab client.
func NewGitLab(ctx context.Context, cfg *gitLabConfig) (*GitLab, error) {
	if cfg.GitLabBaseURL == "" {
		return nil, fmt.Errorf("gitlab base url is required")
	}

	c, err := gitlab.NewClient(cfg.GitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return &GitLab{
		client: c,
		cfg:    cfg,
	}, nil
}

// AssignReviewers adds users to the reviewers list of a Merge Request.
func (g *GitLab) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	return &AssignReviewersResult{}, nil
}

// GetLatestApprovers retrieves the reviewers whose latest review is an
// approval.
func (g *GitLab) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	return &GetLatestApproversResult{}, nil
}

// GetUserRepoPermissions returns a user's access level to a GitLab project.
func (g *GitLab) GetUserRepoPermissions(ctx context.Context) (string, error) {
	return "", nil
}

// GetUserTeamMemberships retrieves a list of groups that a user is a
// member of.
func (g *GitLab) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	return []string{}, nil
}

// GetPolicyData retrieves the required data for policy evaluation.
func (g *GitLab) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	return &GetPolicyDataResult{}, nil
}

// StoragePrefix generates the unique storage prefix for the GitLab platform type.
func (g *GitLab) StoragePrefix(ctx context.Context) (string, error) {
	return "", nil
}

// ListReports lists existing reports for an issue or change request.
func (g *GitLab) ListReports(ctx context.Context, opts *ListReportsOptions) (*ListReportsResult, error) {
	return nil, nil
}

// DeleteReport deletes an existing comment from an issue or change request.
func (g *GitLab) DeleteReport(ctx context.Context, id int64) error {
	return nil
}

// ReportStatus reports the status of a run.
func (g *GitLab) ReportStatus(ctx context.Context, st Status, p *StatusParams) error {
	if err := validateGitLabReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate GitLab reporter inputs: %w", err)
	}

	msg, err := statusMessage(st, p, g.logURL, gitlabMaxCommentLength)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	if err := g.createMergeRequestNote(ctx, msg.String()); err != nil {
		return fmt.Errorf("failed to report status: %w", err)
	}
	return nil
}

// ReportEntrypointsSummary reports the summary for the entrypoints command.
func (g *GitLab) ReportEntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error {
	return nil
}

// ClearReports clears any existing reports that can be removed.
func (g *GitLab) ClearReports(ctx context.Context) error {
	return nil
}

func (g *GitLab) createMergeRequestNote(ctx context.Context, msg string) error {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "creating merge request note",
		"project", g.cfg.GitLabProjectID,
		"merge_request", g.cfg.GitLabMergeRequestID)

	if _, _, err := g.client.Notes.CreateMergeRequestNote(g.cfg.GitLabProjectID, g.cfg.GitLabMergeRequestID, &gitlab.CreateMergeRequestNoteOptions{
		Body: &msg,
	}); err != nil {
		return fmt.Errorf("failed to create merge request note: %w", err)
	}

	return nil
}

func validateGitLabReporterInputs(cfg *gitLabConfig) error {
	var merr error
	if cfg.GitLabProjectID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("gitlab project id is required"))
	}

	if cfg.GitLabMergeRequestID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("gitlab merge request id is required"))
	}

	if cfg.GitLabToken == "" {
		merr = errors.Join(merr, fmt.Errorf("gitlab token is required"))
	}

	return merr
}
