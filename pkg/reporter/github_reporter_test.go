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

package reporter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/testutil"
)

func TestGitHubReporterInputsValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		inputs *GitHubReporterInputs
		err    string
	}{
		{
			name: "success",
			inputs: &GitHubReporterInputs{
				GitHubOwner:             "owner",
				GitHubRepo:              "repo",
				GitHubPullRequestNumber: 1,
				GitHubServerURL:         "https://github.com",
				GitHubRunID:             1,
				GitHubRunAttempt:        1,
				GitHubJob:               "plan (terraform/project1)",
			},
		},
		{
			name:   "error",
			inputs: &GitHubReporterInputs{},
			err: `github owner is required
github repo is required
one of github pull request number or github sha are required`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.inputs.Validate()
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestGitHubReporterStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                   string
		status                 Status
		params                 *StatusParams
		logURL                 string
		expGitHubClientReqs    []*github.Request
		createIssueCommentsErr error
		err                    string
	}{
		{
			name:   "success",
			status: StatusSuccess,
			params: &StatusParams{
				Operation: "plan",
				Dir:       "terraform/project1",
			},
			logURL: "https://github.com",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "#### 🔱 Guardian 🔱 **`PLAN`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
		{
			name:   "error",
			status: StatusSuccess,
			params: &StatusParams{
				Operation: "plan",
				Dir:       "terraform/project1",
			},
			logURL:                 "https://github.com",
			createIssueCommentsErr: fmt.Errorf("FAILED!"),
			err:                    "failed to report: FAILED!",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "#### 🔱 Guardian 🔱 **`PLAN`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gitHubClient := &github.MockGitHubClient{
				CreateIssueCommentsErr: tc.createIssueCommentsErr,
			}

			reporter := &GitHubReporter{
				gitHubClient: gitHubClient,
				inputs: &GitHubReporterInputs{
					GitHubOwner:             "owner",
					GitHubRepo:              "repo",
					GitHubPullRequestNumber: 1,
					GitHubServerURL:         "https://github.com",
					GitHubRunID:             1,
					GitHubRunAttempt:        1,
					GitHubJob:               "plan (terraform/project1)",
				},
				logURL: tc.logURL,
			}

			err := reporter.Status(context.Background(), tc.status, tc.params)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}

func TestGitHubReporterCreateStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                   string
		status                 Status
		params                 *EntrypointsSummaryParams
		logURL                 string
		expGitHubClientReqs    []*github.Request
		createIssueCommentsErr error
		err                    string
	}{
		{
			name:   "success",
			status: StatusSuccess,
			params: &EntrypointsSummaryParams{
				Message:       "summary message",
				UpdateDirs:    []string{"update"},
				DestroyDirs:   []string{"destroy"},
				AbandonedDirs: []string{"abandoned"},
			},
			logURL: "https://github.com",
			expGitHubClientReqs: []*github.Request{
				{
					Name: "CreateIssueComment",
					Params: []any{
						"owner", "repo", int(1), "#### 🔱 Guardian 🔱 [[logs](https://github.com)]\n" +
							"\n" +
							"summary message\n" +
							"\n" +
							"**Update**\n" +
							"update\n" +
							"\n" +
							"**Destroy**\n" +
							"destroy\n" +
							"\n" +
							"**Abandon**\n" +
							"abandoned",
					},
				},
			},
		},
		{
			name:   "error",
			status: StatusSuccess,
			params: &EntrypointsSummaryParams{
				Message:       "summary message",
				UpdateDirs:    []string{"update"},
				DestroyDirs:   []string{"destroy"},
				AbandonedDirs: []string{"abandoned"},
			},
			logURL:                 "https://github.com",
			createIssueCommentsErr: fmt.Errorf("FAILED!"),
			err:                    "failed to report: FAILED!",
			expGitHubClientReqs: []*github.Request{
				{
					Name: "CreateIssueComment",
					Params: []any{
						"owner", "repo", int(1), "#### 🔱 Guardian 🔱 [[logs](https://github.com)]\n" +
							"\n" +
							"summary message\n" +
							"\n" +
							"**Update**\n" +
							"update\n" +
							"\n" +
							"**Destroy**\n" +
							"destroy\n" +
							"\n" +
							"**Abandon**\n" +
							"abandoned",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gitHubClient := &github.MockGitHubClient{
				CreateIssueCommentsErr: tc.createIssueCommentsErr,
			}

			reporter := &GitHubReporter{
				gitHubClient: gitHubClient,
				inputs: &GitHubReporterInputs{
					GitHubOwner:             "owner",
					GitHubRepo:              "repo",
					GitHubPullRequestNumber: 1,
					GitHubServerURL:         "https://github.com",
					GitHubRunID:             1,
					GitHubRunAttempt:        1,
					GitHubJob:               "plan (terraform/project1)",
				},
				logURL: tc.logURL,
			}

			err := reporter.EntrypointsSummary(context.Background(), tc.params)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}

func TestGitHubReporterClearStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		expGitHubClientReqs  []*github.Request
		listIssueCommentsErr error
		err                  string
	}{
		{
			name: "success",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", int(1)},
				},
				{
					Name:   "DeleteIssueComment",
					Params: []any{"owner", "repo", int64(1)},
				},
				{
					Name:   "DeleteIssueComment",
					Params: []any{"owner", "repo", int64(3)},
				},
			},
		},
		{
			name: "error",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", int(1)},
				},
			},
			listIssueCommentsErr: fmt.Errorf("ERROR LISTING!"),
			err:                  "failed to list comments: ERROR LISTING!",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gitHubClient := &github.MockGitHubClient{
				ListIssueCommentResponse: &github.IssueCommentResponse{
					Comments: []*github.IssueComment{
						{ID: 1, Body: githubCommentPrefix + " guardian comment"},
						{ID: 2, Body: "Not a guardian comment"},
						{ID: 3, Body: githubCommentPrefix + " guardian comment"},
					},
				},
				ListIssueCommentsErr: tc.listIssueCommentsErr,
			}

			reporter := &GitHubReporter{
				gitHubClient: gitHubClient,
				inputs: &GitHubReporterInputs{
					GitHubOwner:             "owner",
					GitHubRepo:              "repo",
					GitHubPullRequestNumber: 1,
					GitHubServerURL:         "https://github.com",
					GitHubRunID:             1,
					GitHubRunAttempt:        1,
					GitHubJob:               "plan (terraform/project1)",
				},
			}

			err := reporter.Clear(context.Background())
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}

func TestGitHubReporterOversizeOutput(t *testing.T) {
	t.Parallel()

	t.Run("truncates_message", func(t *testing.T) {
		t.Parallel()

		expGitHubClientReqs := []*github.Request{
			{
				Name:   "CreateIssueComment",
				Params: []any{"owner", "repo", int(1), "#### 🔱 Guardian 🔱 **`PLAN`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1\n\n> Message has been truncated. See workflow logs to view the full message."},
			},
		}

		gitHubClient := &github.MockGitHubClient{}

		reporter := &GitHubReporter{
			gitHubClient: gitHubClient,
			inputs: &GitHubReporterInputs{
				GitHubOwner:             "owner",
				GitHubRepo:              "repo",
				GitHubPullRequestNumber: 1,
				GitHubServerURL:         "https://github.com",
				GitHubRunID:             1,
				GitHubRunAttempt:        1,
				GitHubJob:               "plan (terraform/project1)",
			},
			logURL: "https://github.com",
		}

		err := reporter.Status(context.Background(), StatusSuccess, &StatusParams{
			Operation: "plan",
			Dir:       "terraform/project1",
			Details:   messageOverLimit(),
		})
		if err != nil {
			t.Errorf("unepexted error: %v", err)
		}

		if diff := cmp.Diff(gitHubClient.Reqs, expGitHubClientReqs); diff != "" {
			t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
		}
	})
}

func TestFormatOutputForGitHubDiff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		exp     string
	}{
		{
			name: "replaces_tilde",
			content: `
first section -
first section +
first section ~
first section !

    second section -
    second section +
    second section ~
    second section !
	
- third section
+ third section
~ third section
! third section
	
    - fourth section
    + fourth section
    -/+ fourth section
    +/- fourth section
    ~ fourth section
    ! fourth section`,
			exp: `
first section -
first section +
first section ~
first section !

    second section -
    second section +
    second section ~
    second section !
	
- third section
+ third section
! third section
! third section
	
-     fourth section
+     fourth section
-/+     fourth section
+/-     fourth section
!     fourth section
!     fourth section`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := formatOutputForGitHubDiff(tc.content)
			if got, want := strings.TrimSpace(output), strings.TrimSpace(tc.exp); got != want {
				t.Errorf("expected\n\n%s\n\nto be\n\n%s\n\n", got, want)
			}
		})
	}
}

func messageOverLimit() string {
	message := make([]rune, githubMaxCommentLength)
	for i := 0; i < len(message); i++ {
		message[i] = 'a'
	}
	return string(message)
}
