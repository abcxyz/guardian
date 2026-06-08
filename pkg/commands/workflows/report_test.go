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
	"fmt"
	"testing"

	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/platform"
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
		githubPullRequestNumber        int
		githubSHA                      string
		githubRunID                    int64
		listChangeRequestsByCommitResp *platform.ListChangeRequestsByCommitResponse
		listChangeRequestsByCommitErr  error
		listJobsResp                   []*platform.Job
		listJobsErr                    error
		err                            string
		expPlatformClientReqs          []*platform.Request
	}{
		{
			name:                    "plan_success",
			flagType:                "plan",
			githubPullRequestNumber: 123,
			githubRunID:             456,
			listJobsResp: []*platform.Job{
				{ID: 11, Name: "job1", URL: "url1"},
			},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
				},
			},
		},
		{
			name:        "apply_success_resolves_pr_by_commit",
			flagType:    "apply",
			githubSHA:   "abcdef123456",
			githubRunID: 789,
			listChangeRequestsByCommitResp: &platform.ListChangeRequestsByCommitResponse{
				PullRequests: []*platform.PullRequest{
					{Number: 123},
				},
			},
			listJobsResp: []*platform.Job{
				{ID: 22, Name: "job2", URL: "url2"},
			},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListChangeRequestsByCommit",
					Params: []any{"abcdef123456", (*platform.ListChangeRequestsByCommitOptions)(nil)},
				},
				{
					Name:   "ListJobs",
					Params: []any{int64(789)},
				},
			},
		},
		{
			name:      "apply_no_associated_pr_skips_reporting",
			flagType:  "apply",
			githubSHA: "abcdef123456",
			listChangeRequestsByCommitResp: &platform.ListChangeRequestsByCommitResponse{
				PullRequests: []*platform.PullRequest{},
			},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListChangeRequestsByCommit",
					Params: []any{"abcdef123456", (*platform.ListChangeRequestsByCommitOptions)(nil)},
				},
			},
		},
		{
			name:                          "apply_pr_resolution_fails_errors",
			flagType:                      "apply",
			githubSHA:                     "abcdef123456",
			listChangeRequestsByCommitErr: fmt.Errorf("network error"),
			err:                           "failed to list pull requests for commit [abcdef123456]: network error",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListChangeRequestsByCommit",
					Params: []any{"abcdef123456", (*platform.ListChangeRequestsByCommitOptions)(nil)},
				},
			},
		},
		{
			name:                    "list_jobs_fails_errors",
			flagType:                "plan",
			githubPullRequestNumber: 123,
			githubRunID:             456,
			listJobsErr:             fmt.Errorf("api error"),
			err:                     "failed to list jobs for run [456]: api error",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "ListJobs",
					Params: []any{int64(456)},
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
			}

			c := &ReportCommand{
				flagType:       tc.flagType,
				platformClient: mockPlatformClient,
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
