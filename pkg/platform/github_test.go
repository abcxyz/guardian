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

package platform

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/testutil"
)

func TestGitHub_AssignReviewers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		users               []string
		teams               []string
		requestReviewersErr error
		wantErr             string
		wantResult          *AssignReviewersResult
		expClientReqs       []*github.Request
	}{
		{
			name:  "calls_users_and_teams",
			teams: []string{"test-team-name"},
			users: []string{"test-user-name", "test-user-name-2"},
			expClientReqs: []*github.Request{
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
			wantResult: &AssignReviewersResult{
				Users: []string{"test-user-name", "test-user-name-2"},
				Teams: []string{"test-team-name"},
			},
		},
		{
			name:  "calls_with_only_users",
			users: []string{"test-user-name"},
			expClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string{"test-user-name"}, []string(nil)},
				},
			},
			wantResult: &AssignReviewersResult{
				Users: []string{"test-user-name"},
				Teams: []string(nil),
			},
		},
		{
			name:  "calls_with_only_teams",
			teams: []string{"test-team-name"},
			expClientReqs: []*github.Request{
				{
					Name:   "RequestReviewers",
					Params: []any{"test-owner", "test-repo", 1, []string(nil), []string{"test-team-name"}},
				},
			},
			wantResult: &AssignReviewersResult{
				Users: []string(nil),
				Teams: []string{"test-team-name"},
			},
		},
		{
			name:                "returns_error",
			teams:               []string{"test-team-name"},
			users:               []string{"test-user-name"},
			requestReviewersErr: fmt.Errorf("failed"),
			wantErr:             "failed",
			expClientReqs: []*github.Request{
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
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			client := github.MockGitHubClient{
				RequestReviewersErr: tc.requestReviewersErr,
				UserReviewers:       tc.users,
				TeamReviewers:       tc.teams,
			}
			platform := &GitHub{
				client: &client,
				cfg: &github.Config{
					GitHubOwner:             "test-owner",
					GitHubRepo:              "test-repo",
					GitHubPullRequestNumber: 1,
				},
			}

			if len(tc.users) > 0 {
				client.UserReviewers = tc.users
			}
			if len(tc.teams) > 0 {
				client.TeamReviewers = tc.teams
			}

			res, err := platform.AssignReviewers(ctx, &AssignReviewersInput{
				Users: tc.users,
				Teams: tc.teams,
			})
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(client.Reqs, tc.expClientReqs); diff != "" {
				t.Errorf("RequestReviewers call not as expected; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(res, tc.wantResult); diff != "" {
				t.Errorf("RequestReviewers got unexpected result (-got, want):\n%s", diff)
			}
		})
	}
}
