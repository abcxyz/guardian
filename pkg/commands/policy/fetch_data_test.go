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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestFetchData_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name             string
		isPullRequest    bool
		getPolicyDataErr error
		wantErr          string
		teams            []string
		users            []string
		userAccessLevel  string
		teamMappings     map[string][]string
		want             platform.GetPolicyDataResult
	}{
		{
			name:          "prints_teams_and_users",
			isPullRequest: true,
			teams:         []string{"team1", "team2"},
			users:         []string{"user1", "user2"},
			want: platform.GetPolicyDataResult{
				Mock: &platform.MockPolicyData{
					Approvers: &platform.GetLatestApproversResult{
						Teams: []string{"team1", "team2"},
						Users: []string{"user1", "user2"},
					},
				},
			},
		},
		{
			name:            "prints_user_access_level",
			isPullRequest:   true,
			userAccessLevel: "admin",
			want: platform.GetPolicyDataResult{
				Mock: &platform.MockPolicyData{
					Approvers:       &platform.GetLatestApproversResult{},
					UserAccessLevel: "admin",
				},
			},
		},
		{
			name: "prints_team_memberships",
			teamMappings: map[string][]string{
				"team1": {"user1", "user2"},
				"team2": {"user3", "user4"},
			},
			want: platform.GetPolicyDataResult{
				Mock: &platform.MockPolicyData{
					TeamMemberships: map[string][]string{
						"team1": {"user1", "user2"},
						"team2": {"user3", "user4"},
					},
				},
			},
		},
		{
			name:          "prints_no_approvers",
			isPullRequest: true,
			teams:         []string{},
			users:         []string{},
			want: platform.GetPolicyDataResult{
				Mock: &platform.MockPolicyData{
					Approvers: &platform.GetLatestApproversResult{
						Teams: []string{},
						Users: []string{},
					},
				},
			},
		},
		{
			name:            "prints_no_approvers_outside_of_pull_request",
			teams:           []string{},
			users:           []string{},
			isPullRequest:   false,
			userAccessLevel: "read_only",
			want: platform.GetPolicyDataResult{
				Mock: &platform.MockPolicyData{
					UserAccessLevel: "read_only",
				},
			},
		},
		{
			name:             "fails_with_error",
			getPolicyDataErr: fmt.Errorf("failed to get latest approvers"),
			wantErr:          "failed to get latest approvers",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outDir := t.TempDir()
			c := &FetchDataCommand{
				flags: FetchDataFlags{
					flagOutputDir: outDir,
				},
				platform: &platform.MockPlatform{
					GetPolicyDataErr: tc.getPolicyDataErr,
					TeamApprovers:    tc.teams,
					UserApprovers:    tc.users,
					UserAccessLevel:  tc.userAccessLevel,
					IsPullRequest:    tc.isPullRequest,
					TeamMemberships:  tc.teamMappings,
				},
			}
			outFilepath := path.Join(outDir, policyDataFilename)

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}
			if err != nil {
				// Skip rest of checks because the file won't exist.
				return
			}

			fileData, err := os.ReadFile(outFilepath)
			if err != nil {
				t.Fatalf("failed to read policy data file: %v", err)
			}

			var got platform.GetPolicyDataResult
			if err := json.Unmarshal(fileData, &got); err != nil {
				t.Fatalf("failed to unmarshal policy data json: %v", err)
			}

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("unexpected result (-got, +want):\n%s", diff)
			}
		})
	}
}
