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

package workflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v53/github"

	"github.com/abcxyz/guardian/pkg/plansummary"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/testutil"
)

func TestReportCommandFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		flagType         string
		flagEntrypoints  string
		flagArtifactsDir string
		err              string
	}{
		{
			name:            "invalid_type",
			flagType:        "invalid",
			flagEntrypoints: "[]",
			err:             "missing or invalid flag: type must be 'plan' or 'apply'",
		},
		{
			name:            "missing_entrypoints",
			flagType:        "apply",
			flagEntrypoints: "",
			err:             "missing flag: entrypoints is required",
		},
		{
			name:             "missing_artifacts_dir_for_plan",
			flagType:         "plan",
			flagEntrypoints:  "[]",
			flagArtifactsDir: "",
			err:              "missing flag: artifacts-dir is required when type is 'plan'",
		},
		{
			name:             "valid_plan",
			flagType:         "plan",
			flagEntrypoints:  "[]",
			flagArtifactsDir: "./artifacts",
		},
		{
			name:            "valid_apply",
			flagType:        "apply",
			flagEntrypoints: "[]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &ReportCommand{}
			f := c.Flags()

			var args []string
			if tc.flagType != "" {
				args = append(args, "-type", tc.flagType)
			}
			if tc.flagEntrypoints != "" {
				args = append(args, "-entrypoints", tc.flagEntrypoints)
			}
			if tc.flagArtifactsDir != "" {
				args = append(args, "-artifacts-dir", tc.flagArtifactsDir)
			}

			err := f.Parse(args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestReportCommandProcess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	cases := []struct {
		name                           string
		flagType                       string
		flagEntrypoints                string
		flagArtifactsDir               func(t *testing.T) string
		githubPullRequestNumber        int
		githubSHA                      string
		githubRunID                    int64
		listChangeRequestsByCommitResp *platform.ListChangeRequestsByCommitResponse
		listChangeRequestsByCommitErr  error
		listJobsResp                   []*platform.Job
		listJobsErr                    error
		listReportsResp                []*platform.Report
		listReportsErr                 error
		createReportErr                error
		err                            string
		expPlatformClientReqs          []*platform.Request
	}{
		{
			name:                    "plan_success_posts_table_and_deletes_old_comments",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writePlanSummary(t, filepath.Join(dir, "plan-summary-terraform-github-abseil"), &plansummary.Summary{
					Entrypoint: "terraform/github/abseil",
					Added:      2,
					Changed:    1,
					Deleted:    2,
				})
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "success"},
			},
			listReportsResp: []*platform.Report{
				{ID: int64(999), Body: "#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\nOld comment content"},
				{ID: int64(888), Body: "Normal comment"},
			},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name:   "DeleteReport",
					Params: []any{int64(999)},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n" +
							"| `terraform/github/abseil` | <span style=\"white-space: nowrap;\">🟩&nbsp;SUCCESS</span> | <span style=\"white-space: nowrap;\">+2&nbsp;~1&nbsp;-2</span> | - | <a href=\"job_url_11\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
		{
			name:                    "plan_success_with_org_coalescing",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/google-gh-automation/repositories/auto-fairy"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writePlanSummary(t, filepath.Join(dir, "plan-summary-terraform-google-gh-automation-repositories-auto-fairy"), &plansummary.Summary{
					Entrypoint: "terraform/google-gh-automation/repositories/auto-fairy",
					Added:      1,
					Changed:    0,
					Deleted:    0,
				})
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (google-gh-automation)", URL: "job_url_11", Conclusion: "success"},
			},
			listReportsResp: []*platform.Report{},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n" +
							"| `terraform/google-gh-automation/repositories/auto-fairy` | <span style=\"white-space: nowrap;\">🟩&nbsp;SUCCESS</span> | <span style=\"white-space: nowrap;\">+1&nbsp;~0&nbsp;-0</span> | - | <a href=\"job_url_11\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
		{
			name:                    "plan_matches_summary_by_entrypoint_not_directory",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writePlanSummary(t, filepath.Join(dir, "some", "other", "dir"), &plansummary.Summary{
					Entrypoint: "./terraform/github/abseil/",
					Added:      1,
				})
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "success"},
			},
			listReportsResp: []*platform.Report{},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n" +
							"| `terraform/github/abseil` | <span style=\"white-space: nowrap;\">🟩&nbsp;SUCCESS</span> | <span style=\"white-space: nowrap;\">+1&nbsp;~0&nbsp;-0</span> | - | <a href=\"job_url_11\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
		{
			name:                    "plan_failed_job_missing_artifacts_dir",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "failure"},
			},
			listReportsResp: []*platform.Report{},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n" +
							"| `terraform/github/abseil` | <span style=\"white-space: nowrap;\">🟥&nbsp;FAILED</span> | - | ⚠️ Missing plan summary | <a href=\"job_url_11\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
		{
			name:                    "plan_rejects_tfplan_json_artifact",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writeArtifactFile(t, filepath.Join(dir, "tfplan-terraform-github-abseil"), "tfplan.json")
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "success"},
			},
			err: "found Terraform plan file",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
			},
		},
		{
			name:                    "plan_rejects_tfplan_binary_artifact",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writePlanSummary(t, filepath.Join(dir, "plan-summary-terraform-github-abseil"), &plansummary.Summary{
					Entrypoint: "terraform/github/abseil",
				})
				writeArtifactFile(t, filepath.Join(dir, "plan-summary-terraform-github-abseil"), "tfplan.binary")
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "success"},
			},
			err: "found Terraform plan file",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
			},
		},
		{
			name:                    "plan_rejects_duplicate_summaries",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/abseil"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writePlanSummary(t, filepath.Join(dir, "a"), &plansummary.Summary{Entrypoint: "terraform/github/abseil"})
				writePlanSummary(t, filepath.Join(dir, "b"), &plansummary.Summary{Entrypoint: "terraform/github/abseil"})
				return dir
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (terraform/github/abseil)", URL: "job_url_11", Conclusion: "success"},
			},
			err: "found multiple plan summaries for entrypoint terraform/github/abseil",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
			},
		},
		{
			name:                    "plan_skipped_suppresses_warning",
			flagType:                "plan",
			flagEntrypoints:         `["terraform/github/google-gh-automation/repositories/auto-fairy"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "plan (google-gh-automation)", URL: "job_url_11", Conclusion: "skipped"},
			},
			listReportsResp: []*platform.Report{},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n" +
							"| `terraform/github/google-gh-automation/repositories/auto-fairy` | <span style=\"white-space: nowrap;\">🟨&nbsp;SKIPPED</span> | - | - | <a href=\"job_url_11\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
		{
			name:                    "plan_truncated_if_too_long",
			flagType:                "plan",
			flagEntrypoints:         `["` + strings.Repeat("a", 60000) + `"]`,
			githubPullRequestNumber: 123,
			githubRunID:             456,
			flagArtifactsDir: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
			listJobsResp:    []*platform.Job{},
			listReportsResp: []*platform.Report{},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
							"| Directory | Status | Stats | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- | :--- |\n\n\n" +
							"> ⚠️ **Notice**: The report was truncated due to the character limit. Please check the full list and output artifacts in GitHub Actions logs.\n",
					},
				},
			},
		},
		{
			name:            "apply_success_posts_table",
			flagType:        "apply",
			flagEntrypoints: `["terraform/github/abseil"]`,
			githubSHA:       "sha123",
			githubRunID:     789,
			listChangeRequestsByCommitResp: &platform.ListChangeRequestsByCommitResponse{
				PullRequests: []*platform.PullRequest{
					{Number: 123},
				},
			},
			listJobsResp: []*platform.Job{
				{ID: 22, Name: "apply (terraform/github/abseil)", URL: "job_url_22", Conclusion: "success"},
			},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListChangeRequestsByCommit",
					Params: []any{"sha123", (*platform.ListChangeRequestsByCommitOptions)(nil)},
				},
				{
					Name:   "ListJobs",
					Params: []any{int64(789)},
				},
				{
					Name: "ListReports",
					Params: []any{
						123,
						&platform.ListReportsOptions{
							GitHub: &github.IssueListCommentsOptions{
								ListOptions: github.ListOptions{
									PerPage: 100,
								},
							},
						},
					},
				},
				{
					Name: "CreateReport",
					Params: []any{
						123,
						"#### 🔱 Guardian 🔱 **`APPLY SUMMARY`**\n\n" +
							"| Directory | Status | Notes | Log |\n" +
							"| :--- | :--- | :--- | :--- |\n" +
							"| `terraform/github/abseil` | <span style=\"white-space: nowrap;\">🟩&nbsp;SUCCESS</span> | - | <a href=\"job_url_22\" target=\"_blank\">View Log</a> |\n",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockPlatformClient := &platform.MockPlatform{
				ListChangeRequestsByCommitResp: tc.listChangeRequestsByCommitResp,
				ListChangeRequestsByCommitErr:  tc.listChangeRequestsByCommitErr,
				ListJobsResp:                   tc.listJobsResp,
				ListJobsErr:                    tc.listJobsErr,
				Reports:                        tc.listReportsResp,
				ListReportsErr:                 tc.listReportsErr,
				CreateReportErr:                tc.createReportErr,
			}

			artifactsDir := ""
			if tc.flagArtifactsDir != nil {
				artifactsDir = tc.flagArtifactsDir(t)
			}

			c := &ReportCommand{
				flagType:         tc.flagType,
				flagEntrypoints:  tc.flagEntrypoints,
				flagArtifactsDir: artifactsDir,
				platformClient:   mockPlatformClient,
			}
			c.platformConfig.GitHub.GitHubPullRequestNumber = tc.githubPullRequestNumber
			c.platformConfig.GitHub.GitHubSHA = tc.githubSHA
			c.platformConfig.GitHub.GitHubRunID = tc.githubRunID

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if diff := cmp.Diff(mockPlatformClient.Reqs, tc.expPlatformClientReqs); diff != "" {
				t.Errorf("MockPlatform requests mismatch (-got,+want):\n%s", diff)
			}
		})
	}
}

func writePlanSummary(tb testing.TB, dir string, s *plansummary.Summary) {
	tb.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatalf("failed to create temp dir: %v", err)
	}
	if err := plansummary.Write(filepath.Join(dir, plansummary.Filename), s); err != nil {
		tb.Fatalf("failed to write plan summary: %v", err)
	}
}

func writeArtifactFile(tb testing.TB, dir, name string) {
	tb.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
		tb.Fatalf("failed to write %s: %v", name, err)
	}
}
