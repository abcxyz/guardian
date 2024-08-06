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

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

const (
	TeamApproverName = "TEST-TEAM-APPROVER-NAME"
	UserApproverName = "TEST-USER-APPROVER-NAME"
)

func TestPolicy_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name      string
		results   *Results
		wantTeams []string
		wantUsers []string
		wantErr   string
	}{
		{
			name: "succeeds_with_sufficient_approvals",
			results: &Results{
				"test_policy_name": &Result{
					MissingApprovals: make([]*MissingApproval, 0),
				},
			},
		},
		{
			name: "fails_with_missing_team_approvals",
			results: &Results{
				"test_policy_name": &Result{
					MissingApprovals: []*MissingApproval{
						{
							AssignTeams: []string{
								TeamApproverName,
							},
							Message: "test-error-message",
						},
					},
				},
			},
			wantTeams: []string{
				TeamApproverName,
			},
			wantErr: "failed: \"test_policy_name\" - test-error-message",
		},
		{
			name: "fails_with_missing_user_approvals",
			results: &Results{
				"test_policy_name": &Result{
					MissingApprovals: []*MissingApproval{
						{
							AssignUsers: []string{UserApproverName},
							Message:     "test-error-message",
						},
					},
				},
			},

			wantUsers: []string{UserApproverName},
			wantErr:   "failed: \"test_policy_name\" - test-error-message",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &PolicyCommand{
				results: tc.results,
			}

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(tc.wantTeams, c.teams); diff != "" {
				t.Errorf("expected teams %v to be %v", c.teams, tc.wantTeams)
			}

			if diff := cmp.Diff(tc.wantUsers, c.users); diff != "" {
				t.Errorf("expected users %v to be %v", c.users, tc.wantUsers)
			}
		})
	}
}
