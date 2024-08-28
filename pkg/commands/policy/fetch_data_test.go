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
	"fmt"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestFetchData_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                 string
		getLatestApproverErr error
		wantErr              string
		teams                []string
		users                []string
		wantOut              string
	}{
		{
			name:    "prints_teams_and_users",
			teams:   []string{"team1", "team2"},
			users:   []string{"user1", "user2"},
			wantOut: `{"users":["user1","user2"],"teams":["team1","team2"]}`,
		},
		{
			name:    "prints_no_approvers",
			teams:   []string{},
			users:   []string{},
			wantOut: `{"users":[],"teams":[]}`,
		},
		{
			name:                 "fails_with_error",
			getLatestApproverErr: fmt.Errorf("failed to get latest approvers"),
			wantErr:              "failed to get latest approvers",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &FetchDataCommand{
				platform: &platform.MockPlatform{
					GetLatestApproversErr: tc.getLatestApproverErr,
					TeamApprovers:         tc.teams,
					UserApprovers:         tc.users,
				},
			}
			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.wantOut); !strings.Contains(got, want) {
				t.Errorf("expected stdout\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}
