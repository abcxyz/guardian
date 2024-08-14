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
				GitHubToken:             "token",
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

			err := tc.inputs.Validate()
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
		status                 Status
		params                 *Params
		logURL                 string
		expGitHubClientReqs    []*github.Request
		createIssueCommentsErr error
		err                    string
	}{
		{
			name:   "success",
			status: StatusSuccess,
			params: &Params{
				Operation: "plan",
				IsDestroy: false,
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
			name:   "success_destroy",
			status: StatusSuccess,
			params: &Params{
				Operation: "plan",
				IsDestroy: true,
				Dir:       "terraform/project1",
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
			name:   "error",
			status: StatusSuccess,
			params: &Params{
				Operation: "plan",
				IsDestroy: false,
				Dir:       "terraform/project1",
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

			err := reporter.CreateStatus(context.Background(), tc.status, tc.params)
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

		err := reporter.CreateStatus(context.Background(), StatusSuccess, &Params{
			Operation: "plan",
			Dir:       "terraform/project1",
			Output:    messageOverLimit(),
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
