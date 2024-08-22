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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestPlanStatusCommentsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		flagInitResult        string
		flagPlanResult        []string
		createStatusErr       error
		err                   string
		expReporterClientReqs []*reporter.Request
	}{
		{
			name:           "success",
			flagInitResult: "success",
			flagPlanResult: []string{"success"},
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusSuccess, &reporter.StatusParams{Operation: "plan", Message: "Plan completed successfully."}},
				},
			},
			err: "",
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
			name:           "indeterminate",
			flagInitResult: "cancelled",
			flagPlanResult: []string{"cancelled"},
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusUnknown, &reporter.StatusParams{Operation: "plan", Message: "Unable to determine plan status."}},
				},
			},
			err: "unable to determine plan status",
		},
		{
			name:           "handles_errors",
			flagInitResult: "success",
			flagPlanResult: []string{"success"},
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusSuccess, &reporter.StatusParams{Operation: "plan", Message: "Plan completed successfully."}},
				},
			},
			createStatusErr: fmt.Errorf("error creating comment"),
			err:             "failed to create plan status comment: error creating comment",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockReporterClient := &reporter.MockReporter{
				StatusErr: tc.createStatusErr,
			}

			c := &PlanStatusCommentCommand{
				flagInitResult: tc.flagInitResult,
				flagPlanResult: tc.flagPlanResult,
				reporterClient: mockReporterClient,
			}

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(mockReporterClient.Reqs, tc.expReporterClientReqs); diff != "" {
				t.Errorf("ReporterClient calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
