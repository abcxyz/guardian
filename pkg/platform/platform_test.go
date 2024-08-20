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

func TestPlatform_AssignReviewers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		users              []string
		teams              []string
		assignReviewersErr error
		wantErr            string
		wantResult         *AssignReviewersResult
		expClientReqs      []*Request
	}{
		{
			name:  "calls_users_and_teams",
			teams: []string{"test-team-name"},
			users: []string{"test-user-name", "test-user-name-2"},
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name", "test-user-name-2"}, []string{"test-team-name"}},
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
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name"}, []string(nil)},
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
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string(nil), []string{"test-team-name"}},
				},
			},
			wantResult: &AssignReviewersResult{
				Users: []string(nil),
				Teams: []string{"test-team-name"},
			},
		},
		{
			name:               "returns_error",
			teams:              []string{"test-team-name"},
			users:              []string{"test-user-name"},
			assignReviewersErr: fmt.Errorf("failed"),
			wantErr:            "failed",
			expClientReqs: []*Request{
				{
					Name:   "AssignReviewers",
					Params: []any{[]string{"test-user-name"}, []string{"test-team-name"}},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			platform := &MockPlatform{
				AssignReviewersErr: tc.assignReviewersErr,
				UserReviewers:      tc.users,
				TeamReviewers:      tc.teams,
			}

			res, err := platform.AssignReviewers(ctx, &AssignReviewersInput{
				Users: tc.users,
				Teams: tc.teams,
			})
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(platform.Reqs, tc.expClientReqs); diff != "" {
				t.Errorf("AssignReviewers call not as expected; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(res, tc.wantResult); diff != "" {
				t.Errorf("AssignReviewers got unexpected result (-got, want):\n%s", diff)
			}
		})
	}
}
