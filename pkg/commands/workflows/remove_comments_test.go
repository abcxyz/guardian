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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/commands/apply"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestRemoveGuardianCommentsAfterParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "validate_github_flags",
			args: []string{"-pull-request-number=1", "-for-command=plan"},
			err:  "missing flag: github-owner is required\nmissing flag: github-repo is required",
		},
		{
			name: "validate_pull_request_number",
			args: []string{"-for-command=plan", "-github-owner=owner", "-github-repo=repo"},
			err:  "missing flag: pull-request-number is required",
		},
		{
			name: "validate_comment_type",
			args: []string{"-pull-request-number=1", "-github-owner=owner", "-github-repo=repo"},
			err:  "missing flag: for-command is required",
		},
		{
			name: "wrong_comment_type",
			args: []string{"-for-command=test", "-for-command=wrong", "-pull-request-number=1", "-github-owner=owner", "-github-repo=repo"},
			err:  "invalid value for-command: test is not one of [\"apply\" \"plan\"]\ninvalid value for-command: wrong is not one of [\"apply\" \"plan\"]",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &RemoveGuardianCommentsCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestRemoveGuardianCommentsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		pullRequestNumber     int
		flagIsGitHubActions   bool
		flagGitHubOwner       string
		flagGitHubRepo        string
		flagPullRequestNumber int
		flagForCommands       []string
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
			flagForCommands:       []string{"plan"},
			gitHubClient: &github.MockGitHubClient{
				ListIssueCommentResponse: &github.IssueCommentResponse{
					Comments: []*github.IssueComment{
						{
							ID:   1,
							Body: plan.CommentPrefix + " comment message",
						},
						{
							ID:   2,
							Body: apply.CommentPrefix + " comment message",
						},
					},
				},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 1},
				},
				{
					Name:   "DeleteIssueComment",
					Params: []any{"owner", "repo", int64(1)},
				},
			},
			err:       "",
			expStdout: "",
			expStderr: "",
		},
		{
			name:                  "handles_errors",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 2,
			flagForCommands:       []string{"plan"},
			gitHubClient: &github.MockGitHubClient{
				ListIssueCommentsErr: fmt.Errorf("error getting comments"),
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 2},
				},
			},
			err:       "failed to list comments: error getting comments",
			expStdout: "",
			expStderr: "",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &RemoveGuardianCommentsCommand{
				GitHubFlags: flags.GitHubFlags{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					FlagGitHubOwner:     tc.flagGitHubOwner,
					FlagGitHubRepo:      tc.flagGitHubRepo,
				},
				flagPullRequestNumber: tc.flagPullRequestNumber,
				flagForCommands:       tc.flagForCommands,
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
