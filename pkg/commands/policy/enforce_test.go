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
	"fmt"
	"testing"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestEnforce_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name               string
		resultsFile        string
		assignReviewersErr error
		wantErrs           []string
	}{
		{
			name:        "succeeds_with_sufficient_approvals",
			resultsFile: "testdata/no_missing_approvals.json",
		},
		{
			name:        "fails_with_missing_team_approvals",
			resultsFile: "testdata/missing_team_approval.json",
			wantErrs:    []string{"failed: \"test_policy_name\" - test-error-message"},
		},
		{
			name:        "fails_with_missing_user_approvals",
			resultsFile: "testdata/missing_user_approval.json",
			wantErrs:    []string{"failed: \"test_policy_name\" - test-error-message"},
		},
		{
			name:               "fails_with_platform_api_error",
			resultsFile:        "testdata/missing_user_approval.json",
			assignReviewersErr: fmt.Errorf("failed to assign reviewers"),
			wantErrs:           []string{"failed to assign reviewers"},
		},
		{
			name:        "fails_with_deny",
			resultsFile: "testdata/deny.json",
			wantErrs:    []string{"test-error-message"},
		},
		{
			name:        "fails_with_deny_and_missing_approvals",
			resultsFile: "testdata/deny_and_missing_approvals.json",
			wantErrs: []string{
				"failed: \"missing_approval_policy\" - violation 2",
				"failed: \"missing_approval_policy\" - violation 4",
				"failed: \"deny_policy\" - violation 1",
				"failed: \"deny_policy\" - violation 3",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &EnforceCommand{
				flags: EnforceFlags{
					ResultsFile: tc.resultsFile,
				},
				platform: &platform.MockPlatform{
					AssignReviewersErr: tc.assignReviewersErr,
				},
			}

			err := c.Process(ctx)
			for _, wantErr := range tc.wantErrs {
				if diff := testutil.DiffErrString(err, wantErr); diff != "" {
					t.Errorf("Process() %s", diff)
				}
			}
		})
	}
}
