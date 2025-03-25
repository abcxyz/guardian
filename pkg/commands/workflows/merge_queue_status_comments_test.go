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

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestMergeQueueStatusCommentsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		flagResult            string
		flagTargetBranch      string
		createStatusErr       error
		err                   string
		expPlatformClientReqs []*platform.Request
	}{
		{
			name:       "success",
			flagResult: github.GitHubWorkflowResultSuccess,
		},
		{
			name:             "failed_status",
			flagResult:       github.GitHubWorkflowResultFailure,
			flagTargetBranch: "main",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusFailure, &platform.StatusParams{Operation: "merge-check", Message: "Your pull request is out of date, please rebase against `main` and resubmit to the merge queue."}},
				},
			},
		},
		{
			name:             "handles_errors",
			flagResult:       github.GitHubWorkflowResultFailure,
			flagTargetBranch: "main",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusFailure, &platform.StatusParams{Operation: "merge-check", Message: "Your pull request is out of date, please rebase against `main` and resubmit to the merge queue."}},
				},
			},
			createStatusErr: fmt.Errorf("error creating comment"),
			err:             "failed to create merge queue status comment: error creating comment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockPlatformClient := &platform.MockPlatform{
				ReportStatusErr: tc.createStatusErr,
			}

			c := &MergeQueueStatusCommentCommand{
				flagResult:       tc.flagResult,
				flagTargetBranch: tc.flagTargetBranch,
				platformClient:   mockPlatformClient,
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
