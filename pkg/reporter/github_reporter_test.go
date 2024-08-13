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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/testutil"
)

func TestGitHubReporterValidateConfig(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		flags flags.GitHubFlags
		err   string
	}{
		{
			name: "success",
			flags: flags.GitHubFlags{
				FlagGitHubToken:             "token",
				FlagGitHubOwner:             "owner",
				FlagGitHubRepo:              "repo",
				FlagGitHubPullRequestNumber: 1,
				FlagGitHubServerURL:         "https://github.com",
				FlagGitHubRunID:             1,
				FlagGitHubRunAttempt:        1,
				FlagGitHubJob:               "plan (terraform/project1)",
			},
		},
		{
			name:  "error",
			flags: flags.GitHubFlags{},
			err: `one of github token or github app id are required
github owner is required
github repo is required
one of github pull request number or github sha are required
github server url is required
github run id is required
github run attempt is required`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateFlags(tc.flags)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestGitHubReporterCreateStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                   string
		params                 *Params
		logURL                 string
		expGitHubClientReqs    []*github.Request
		createIssueCommentsErr error
		err                    string
	}{
		{
			name: "success",
			params: &Params{
				Operation:     PlanOperation,
				Status:        SuccessStatus,
				IsDestroy:     false,
				EntrypointDir: "terraform/project1",
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
			name: "success_destroy",
			params: &Params{
				Operation:     PlanOperation,
				Status:        SuccessStatus,
				IsDestroy:     true,
				EntrypointDir: "terraform/project1",
			},
			logURL: "https://github.com",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "#### 🔱 Guardian 🔱 **`PLAN`** **`💥 DESTROY`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
		{
			name: "error",
			params: &Params{
				Operation:     PlanOperation,
				Status:        SuccessStatus,
				IsDestroy:     false,
				EntrypointDir: "terraform/project1",
			},
			logURL:                 "https://github.com",
			createIssueCommentsErr: fmt.Errorf("FAILED!"),
			err:                    "failed to report: FAILED!",
			expGitHubClientReqs: []*github.Request{
				{
					Name: "CreateIssueComment",
					// Params: []any{"owner", "repo", int(1), GitHubCommentPrefix + " " + markdownPill(PlanOperationText) + " " + markdownPill(SuccessStatusText) + " [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
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
				flags: flags.GitHubFlags{
					FlagGitHubOwner:             "owner",
					FlagGitHubRepo:              "repo",
					FlagGitHubPullRequestNumber: 1,
					FlagGitHubServerURL:         "https://github.com",
					FlagGitHubRunID:             1,
					FlagGitHubRunAttempt:        1,
					FlagGitHubJob:               "plan (terraform/project1)",
				},
				logURL: tc.logURL,
			}

			err := reporter.CreateStatus(context.Background(), tc.params)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}

func TestGitHubReporterUpdateStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                   string
		params                 *Params
		logURL                 string
		expGitHubClientReqs    []*github.Request
		updateIssueCommentsErr error
		err                    string
	}{
		{
			name: "success",
			params: &Params{
				Operation:     ApplyOperation,
				Status:        SuccessStatus,
				IsDestroy:     false,
				EntrypointDir: "terraform/project1",
			},
			logURL: "https://github.com",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "#### 🔱 Guardian 🔱 **`APPLY`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
		{
			name: "success_destroy",
			params: &Params{
				Operation:     ApplyOperation,
				Status:        FailureStatus,
				IsDestroy:     true,
				EntrypointDir: "terraform/project1",
			},
			logURL: "https://github.com",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "#### 🔱 Guardian 🔱 **`APPLY`** **`💥 DESTROY`** **`🟥 FAILED`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
		{
			name: "error",
			params: &Params{
				Operation:     ApplyOperation,
				Status:        FailureStatus,
				IsDestroy:     false,
				EntrypointDir: "terraform/project1",
			},
			logURL:                 "https://github.com",
			updateIssueCommentsErr: fmt.Errorf("FAILED!"),
			err:                    "failed to report: FAILED!",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "#### 🔱 Guardian 🔱 **`APPLY`** **`🟥 FAILED`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1"},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gitHubClient := &github.MockGitHubClient{
				UpdateIssueCommentsErr: tc.updateIssueCommentsErr,
			}

			reporter := &GitHubReporter{
				gitHubClient: gitHubClient,
				flags: flags.GitHubFlags{
					FlagGitHubOwner:             "owner",
					FlagGitHubRepo:              "repo",
					FlagGitHubPullRequestNumber: 1,
					FlagGitHubServerURL:         "https://github.com",
					FlagGitHubRunID:             1,
					FlagGitHubRunAttempt:        1,
					FlagGitHubJob:               "plan (terraform/project1)",
				},
				logURL:    tc.logURL,
				commentID: int64(1),
			}

			err := reporter.UpdateStatus(context.Background(), tc.params)
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
				Name:   "UpdateIssueComment",
				Params: []any{"owner", "repo", int64(1), "#### 🔱 Guardian 🔱 **`PLAN`** **`🟩 SUCCESS`** [[logs](https://github.com)]\n\n**Entrypoint:** terraform/project1\n\n> Message has been truncated. See workflow logs to view the full message."},
			},
		}

		gitHubClient := &github.MockGitHubClient{}

		reporter := &GitHubReporter{
			gitHubClient: gitHubClient,
			flags: flags.GitHubFlags{
				FlagGitHubOwner:             "owner",
				FlagGitHubRepo:              "repo",
				FlagGitHubPullRequestNumber: 1,
				FlagGitHubServerURL:         "https://github.com",
				FlagGitHubRunID:             1,
				FlagGitHubRunAttempt:        1,
				FlagGitHubJob:               "plan (terraform/project1)",
			},
			logURL:    "https://github.com",
			commentID: int64(1),
		}

		err := reporter.UpdateStatus(context.Background(), &Params{
			Operation:     PlanOperation,
			Status:        SuccessStatus,
			EntrypointDir: "terraform/project1",
			Output:        messageOverLimit(),
		})
		if err != nil {
			t.Errorf("unepexted error: %v", err)
		}

		if diff := cmp.Diff(gitHubClient.Reqs, expGitHubClientReqs); diff != "" {
			t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
		}
	})
}

func messageOverLimit() string {
	message := make([]rune, githubMaxCommentLength)
	for i := 0; i < len(message); i++ {
		message[i] = 'a'
	}
	return string(message)
}
