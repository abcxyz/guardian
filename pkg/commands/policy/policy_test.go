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

package policy

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

const (
	testOwner             = "test-owner"
	testRepo              = "test-repo"
	testPullRequestNumber = 1
)

func TestPolicy_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                string
		resultsFile         string
		wantErr             string
		expGitHubClientReqs []*github.Request
	}{
		{
			name:        "succeeds_with_sufficient_approvals",
			resultsFile: "testdata/no_missing_approvals.json",
		},
		{
			name:        "fails_with_missing_team_approvals",
			resultsFile: "testdata/missing_team_approval.json",
			wantErr:     "failed: \"test_policy_name\" - test-error-message",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name:        "fails_with_missing_user_approvals",
			resultsFile: "testdata/missing_user_approval.json",
			wantErr:     "failed: \"test_policy_name\" - test-error-message",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string{"test-user-name"}, []string(nil)},
				},
			},
		},
		{
			name:        "makes_one_request_per_principal",
			resultsFile: "testdata/missing_user_and_team_approval.json",
			wantErr:     "failed: \"test_policy_name\" - test-error-message",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string{"test-user-name"}, []string(nil)},
				},
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string(nil), []string{"test-team-name"}},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gitHubClient github.MockGitHubClient
			c := &PolicyCommand{
				flags: PolicyFlags{
					ResultsFile: tc.resultsFile,
				},
				codeReview: &GitHub{
					client: &gitHubClient,
					params: &GitHubParams{
						Owner:             testOwner,
						Repository:        testRepo,
						PullRequestNumber: testPullRequestNumber,
					},
				},
			}

			err := c.Process(ctx)

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}

			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}
		})
	}
}
