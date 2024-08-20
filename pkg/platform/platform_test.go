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

	"github.com/abcxyz/pkg/testutil"
)

func TestGitHub_AssignReviewers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		assignReviewersInput *AssignReviewersInput
		users                []string
		teams                []string
		assignReviewersErr   error
		wantErr              string
		expClientReqs        []*Request
	}{
		{
			name: "calls_users_and_teams",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
				Users: []string{"test-user-name", "test-user-name-2"},
			},
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name", "test-user-name-2"}, []string{"test-team-name"}},
				},
			},
		},
		{
			name: "calls_with_only_users",
			assignReviewersInput: &AssignReviewersInput{
				Users: []string{"test-user-name"},
			},
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name"}, []string(nil)},
				},
			},
		},
		{
			name: "calls_with_only_teams",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
			},
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string(nil), []string{"test-team-name"}},
				},
			},
		},
		{
			name: "returns_error",
			assignReviewersInput: &AssignReviewersInput{
				Teams: []string{"test-team-name"},
				Users: []string{"test-user-name"},
			},
			assignReviewersErr: fmt.Errorf("failed to assign all requested reviewers to pull request"),
			wantErr:            "failed to assign all requested reviewers to pull request",
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name"}, []string{"test-team-name"}},
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

			platform := &MockPlatform{
				AssignReviewersErr: tc.assignReviewersErr,
			}

			_, err := platform.AssignReviewers(ctx, tc.assignReviewersInput)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(platform.Reqs, tc.expClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
