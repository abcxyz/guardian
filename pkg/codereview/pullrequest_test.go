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

package codereview

import (
	"context"
	"fmt"
	"testing"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

const (
	testOwner             = "test-owner"
	testRepo              = "test-repo"
	testPullRequestNumber = 1
)

func TestPullRequest_AssignReviewers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		users               []string
		teams               []string
		requestReviewersErr error
		wantErr             string
		expGitHubClientReqs []*github.Request
	}{
		{
			name:  "calls_with_users_and_teams",
			users: []string{"test-user-name"},
			teams: []string{"test-team-name"},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string{"test-user-name"}, []string{"test-team-name"}},
				},
			},
		},
		{
			name:  "calls_with_only_users",
			users: []string{"test-user-name"},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string{"test-user-name"}, []string(nil)},
				},
			},
		},
		{
			name:  "calls_with_only_teams",
			teams: []string{"test-team-name"},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name:                "returns_error",
			users:               []string{"test-user-name"},
			teams:               []string{"test-team-name"},
			requestReviewersErr: fmt.Errorf("failed"),
			wantErr:             "failed to assign reviewers to pull request: failed",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{testOwner, testRepo, testPullRequestNumber, []string{"test-user-name"}, []string{"test-team-name"}},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			gitHubClient := &github.MockGitHubClient{
				RequestReviewersErr: tc.requestReviewersErr,
			}
			pr := &PullRequest{
				client: gitHubClient,
				params: &PullRequestInput{
					Owner:             testOwner,
					Repository:        testRepo,
					PullRequestNumber: testPullRequestNumber,
				},
			}

			err := pr.AssignReviewers(ctx, &AssignReviewersInput{
				Teams: tc.teams,
				Users: tc.users,
			})
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
