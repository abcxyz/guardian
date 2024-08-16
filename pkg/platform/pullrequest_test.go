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

package platform

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/testutil"
)

func TestPullRequest_AssignReviewers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		assignReviewersInput *AssignReviewersInput
		users                []string
		teams                []string
		requestReviewersErr  error
		wantErr              string
		expGitHubClientReqs  []*github.Request
	}{
		{
			name: "calls_users_and_teams_individually",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
				Users: []string{"test-user-name", "test-user-name-2"},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string{"test-user-name"}, []string(nil)},
				},
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string{"test-user-name-2"}, []string(nil)},
				},
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name: "calls_with_only_users",
			assignReviewersInput: &AssignReviewersInput{
				Users: []string{"test-user-name"},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string{"test-user-name"}, []string(nil)},
				},
			},
		},
		{
			name: "calls_with_only_teams",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name: "returns_api_error",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
				Users: []string{"test-user-name"},
			},
			requestReviewersErr: fmt.Errorf("failed"),
			wantErr:             "failed to assign all requested reviewers to pull request",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string{"test-user-name"}, []string(nil)},
				},
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name:    "returns_error_with_missing_inputs",
			wantErr: "inputs cannot be nil",
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
					Owner:             "test-owner",
					Repository:        "test-repo",
					PullRequestNumber: 1,
				},
			}

			_, err := pr.AssignReviewers(ctx, tc.assignReviewersInput)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
