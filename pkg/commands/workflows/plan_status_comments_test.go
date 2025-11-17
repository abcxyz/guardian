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

package workflows

import (
	"fmt"
	"testing"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestPlanStatusCommentsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		flagInitResult        string
		flagPlanResult        []string
		createStatusErr       error
		err                   string
		expPlatformClientReqs []*platform.Request
	}{
		{
			name:           "success",
			flagInitResult: "success",
			flagPlanResult: []string{"success"},
		},
		{
			name:           "skipped",
			flagInitResult: "success",
			flagPlanResult: []string{"skipped", "skipped"},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusNoOperation, &platform.StatusParams{Operation: "plan", Message: "No Terraform changes detected, planning skipped."}},
				},
			},
		},
		{
			name:           "multi_failure",
			flagInitResult: "success",
			flagPlanResult: []string{"success", "failure"},
			err:            "init or plan has one or more failures",
		},
		{
			name:           "failure",
			flagInitResult: "failure",
			flagPlanResult: []string{"success"},
			err:            "init or plan has one or more failures",
		},
		{
			name:           "handles_errors",
			flagInitResult: "success",
			flagPlanResult: []string{"skipped", "skipped"},
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusNoOperation, &platform.StatusParams{Operation: "plan", Message: "No Terraform changes detected, planning skipped."}},
				},
			},
			createStatusErr: fmt.Errorf("error creating comment"),
			err:             "failed to create plan status comment: error creating comment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockPlatformClient := &platform.MockPlatform{
				ReportStatusErr: tc.createStatusErr,
			}

			c := &PlanStatusCommentCommand{
				flagInitResult: tc.flagInitResult,
				flagPlanResult: tc.flagPlanResult,
				platformClient: mockPlatformClient,
			}

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if diff := cmp.Diff(mockPlatformClient.Reqs, tc.expPlatformClientReqs); diff != "" {
				t.Errorf("ReporterClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
