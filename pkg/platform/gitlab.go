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
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

var (
	_ Platform = (*GitLab)(nil)

	// gitLabIgnoredStatusCodes are status codes that should not be retried. This
	// list is taken from the GitLab REST API documentation but may not contain
	// the full set of status codes to ignore.
	// See https://docs.gitlab.com/ee/api/rest/troubleshooting.html#status-codes.
	gitLabIgnoredStatusCodes = map[int]struct{}{
		403: {},
		405: {},
		422: {},
	}
)

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
	// Retry
	MaxRetries        uint64
	InitialRetryDelay time.Duration
	MaxRetryDelay     time.Duration

	GuardianGitLabToken string
	GitLabBaseURL       string

	GitLabProjectID       int
	GitLabMergeRequestIID int
}

type gitLabPredefinedConfig struct {
	CIJobToken   string
	CIServerHost string
	CIProjectID  int
	// The merge request IID is the number used in the GitLab API, and not ID.
	// See https://docs.gitlab.com/ee/ci/variables/predefined_variables.html.
	CIMergeRequestIID int
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

	if v, err := strconv.Atoi(os.Getenv("CI_MERGE_REQUEST_IID")); err == nil {
		c.CIMergeRequestIID = v
	}
}

func (c *gitLabConfig) RegisterFlags(set *cli.FlagSet) {
	f := set.NewSection("GITLAB OPTIONS")

	cfgDefaults := &gitLabPredefinedConfig{}
	cfgDefaults.Load()

	f.StringVar(&cli.StringVar{
		Name:    "guardian-gitlab-token",
		EnvVar:  "GUARDIAN_GITLAB_TOKEN",
		Target:  &c.GuardianGitLabToken,
		Default: cfgDefaults.CIJobToken,
		Usage:   "The GitLab access token to make GitLab API calls.",
		Hidden:  true,
	})

	f.StringVar(&cli.StringVar{
		Name:    "gitlab-base-url",
		EnvVar:  "GITLAB_BASE_URL",
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
		Name:    "gitlab-merge-request-iid",
		EnvVar:  "GITLAB_MERGE_REQUEST_IID",
		Target:  &c.GitLabMergeRequestIID,
		Default: cfgDefaults.CIMergeRequestIID,
		Usage:   "The GitLab project-level merge request internal ID.",
		Hidden:  true,
	})
}

// NewGitLab creates a new GitLab client.
func NewGitLab(ctx context.Context, cfg *gitLabConfig) (*GitLab, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialRetryDelay <= 0 {
		cfg.InitialRetryDelay = 1 * time.Second
	}
	if cfg.MaxRetryDelay <= 0 {
		cfg.MaxRetryDelay = 20 * time.Second
	}

	if cfg.GitLabBaseURL == "" {
		return nil, fmt.Errorf("gitlab base url is required")
	}

	c, err := gitlab.NewClient(cfg.GuardianGitLabToken, gitlab.WithBaseURL(cfg.GitLabBaseURL))
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
	if err := validateGitLabReporterInputs(g.cfg); err != nil {
		return nil, fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	var reports []*Report
	var pagination *Pagination

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		notes, resp, err := g.client.Notes.ListMergeRequestNotes(g.cfg.GitLabProjectID, g.cfg.GitLabMergeRequestIID, opts.GitLab)
		if err != nil {
			if resp != nil {
				if _, ok := gitLabIgnoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
		}
		for _, n := range notes {
			reports = append(reports, &Report{ID: n.ID, Body: n.Body})
		}

		if resp.NextPage != 0 {
			pagination = &Pagination{NextPage: resp.NextPage}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list reports: %w", err)
	}

	return &ListReportsResult{Reports: reports, Pagination: pagination}, nil
}

// DeleteReport deletes an existing comment from an issue or change request.
func (g *GitLab) DeleteReport(ctx context.Context, id any) error {
	if err := validateGitLabReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	noteID, ok := id.(int)
	if !ok {
		return fmt.Errorf("expected note id of type int")
	}

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		resp, err := g.client.Notes.DeleteMergeRequestNote(g.cfg.GitLabProjectID, g.cfg.GitLabMergeRequestIID, noteID)
		if err != nil {
			if resp != nil {
				if _, ok := gitLabIgnoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to delete merge request note: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to delete report: %w", err)
	}

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
func (g *GitLab) ReportEntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	if err := validateGitLabReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	msg, err := entrypointsSummaryMessage(p, g.logURL)
	if err != nil {
		return fmt.Errorf("failed to generate summary message: %w", err)
	}

	if err := g.createMergeRequestNote(ctx, msg.String()); err != nil {
		return fmt.Errorf("failed to report entrypoints summary: %w", err)
	}

	return nil
}

// ClearReports clears any existing reports that can be removed.
func (g *GitLab) ClearReports(ctx context.Context) error {
	if err := validateGitLabReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	listOpts := &ListReportsOptions{
		GitLab: &gitlab.ListMergeRequestNotesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100},
		},
	}

	for {
		response, err := g.ListReports(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Reports == nil {
			return nil
		}

		for _, note := range response.Reports {
			// prefix is not found, skip
			if !strings.HasPrefix(note.Body, commentPrefix) {
				continue
			}

			// found the prefix, delete the comment
			if err := g.DeleteReport(ctx, note.ID); err != nil {
				return fmt.Errorf("failed to delete comment: %w", err)
			}
		}

		if response.Pagination == nil {
			return nil
		}
		listOpts.GitLab.Page = response.Pagination.NextPage
	}
}

func (g *GitLab) createMergeRequestNote(ctx context.Context, msg string) error {
	logger := logging.FromContext(ctx)

	logger.DebugContext(ctx, "creating merge request note",
		"project", g.cfg.GitLabProjectID,
		"merge_request", g.cfg.GitLabMergeRequestIID)

	if _, _, err := g.client.Notes.CreateMergeRequestNote(g.cfg.GitLabProjectID, g.cfg.GitLabMergeRequestIID, &gitlab.CreateMergeRequestNoteOptions{
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

	if cfg.GitLabMergeRequestIID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("gitlab merge request id is required"))
	}

	if cfg.GuardianGitLabToken == "" {
		merr = errors.Join(merr, fmt.Errorf("gitlab token is required"))
	}

	return merr
}

func (g *GitLab) withRetries(ctx context.Context, retryFunc retry.RetryFunc) error {
	backoff := retry.NewFibonacci(g.cfg.InitialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.MaxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.MaxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, retryFunc); err != nil {
		return fmt.Errorf("failed to execute retriable function: %w", err)
	}
	return nil
}

// ListChangeRequestsByCommit lists the merge requests associated with a commit SHA.
func (g *GitLab) ListChangeRequestsByCommit(ctx context.Context, sha string) (any, error) {
	return nil, nil
}
