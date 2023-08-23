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
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestPlanStatusCommentsAfterParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "validate_github_flags",
			args: []string{"-pull-request-number=1", "-init-result=success", "-plan-result=success"},
			err:  "missing flag: github-owner is required\nmissing flag: github-repo is required",
		},
		{
			name: "validate_pull_request_number",
			args: []string{"-github-owner=owner", "-github-repo=repo", "-init-result=success", "-plan-result=success"},
			err:  "missing flag: pull-request-number is required",
		},
		{
			name: "validate_init_result",
			args: []string{"-github-owner=owner", "-github-repo=repo", "-pull-request-number=1", "-plan-result=success"},
			err:  "missing flag: init-result is required",
		},
		{
			name: "validate_plan_result",
			args: []string{"-github-owner=owner", "-github-repo=repo", "-pull-request-number=1", "-init-result=success"},
			err:  "missing flag: plan-result is required",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &PlanStatusCommentsCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestPlanStatusCommentsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	defaultConfig := &PlanStatusCommentsConfig{
		ServerURL:  "https://github.com",
		RunID:      int64(100),
		RunAttempt: int64(1),
	}

	cases := []struct {
		name                  string
		pullRequestNumber     int
		flagIsGitHubActions   bool
		flagGitHubOwner       string
		flagGitHubRepo        string
		flagPullRequestNumber int
		flagInitResult        string
		flagPlanResult        string
		gitHubClient          *github.MockGitHubClient
		err                   string
		expGitHubClientReqs   []*github.Request
		expStdout             string
		expStderr             string
	}{
		{
			name:                  "success",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 1,
			flagInitResult:        "success",
			flagPlanResult:        "success",
			gitHubClient:          &github.MockGitHubClient{},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", 1, "**`🔱 Guardian 🔱 PLAN`** - 🟩 Plan completed successfully. [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
			},
			err:       "",
			expStdout: "",
			expStderr: "",
		},
		{
			name:                  "failure",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 2,
			flagInitResult:        "failure",
			flagPlanResult:        "failure",
			gitHubClient:          &github.MockGitHubClient{},
			expGitHubClientReqs:   nil,
			err:                   "init or plan has one or more failures",
			expStdout:             "",
			expStderr:             "",
		},
		{
			name:                  "indeterminate",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagInitResult:        "cancelled",
			flagPlanResult:        "skipped",
			gitHubClient:          &github.MockGitHubClient{},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", 3, "**`🔱 Guardian 🔱 PLAN`** - 🟨 Unable to determine plan status. [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
			},
			err:       "unable to determine plan status, init and/or plan was skipped or cancelled",
			expStdout: "",
			expStderr: "",
		},
		{
			name:                  "handles_errors",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 4,
			flagInitResult:        "success",
			flagPlanResult:        "success",
			gitHubClient: &github.MockGitHubClient{
				CreateIssueCommentsErr: fmt.Errorf("error creating comment"),
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", 4, "**`🔱 Guardian 🔱 PLAN`** - 🟩 Plan completed successfully. [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
			},
			err:       "failed to create plan status comment: error creating comment",
			expStdout: "",
			expStderr: "",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actions := githubactions.New(githubactions.WithWriter(os.Stdout))

			c := &PlanStatusCommentsCommand{
				cfg: defaultConfig,

				GitHubFlags: flags.GitHubFlags{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					FlagGitHubOwner:     tc.flagGitHubOwner,
					FlagGitHubRepo:      tc.flagGitHubRepo,
				},
				flagPullRequestNumber: tc.flagPullRequestNumber,
				flagInitResult:        tc.flagInitResult,
				flagPlanResult:        tc.flagPlanResult,
				actions:               actions,
				gitHubClient:          tc.gitHubClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(tc.gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected stdout\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected stderr\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}
