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

package plan

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

var terraformNoDiffMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	PlanResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform plan success - no diff",
		ExitCode: 0,
	},
	ShowResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform show success - no diff",
		ExitCode: 0,
	},
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success - no diff"}`,
		ExitCode: 0,
	},
}

var terraformDiffMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success with diff",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success with diff",
		ExitCode: 0,
	},
	PlanResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform plan success with diff",
		ExitCode: 2,
	},
	ShowResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform show success with diff",
		ExitCode: 0,
	},
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success with diff"}`,
		ExitCode: 0,
	},
}

var terraformErrorMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init output",
		Stderr:   "terraform init failed",
		ExitCode: 1,
		Err:      fmt.Errorf("failed to run terraform init"),
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	PlanResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform plan success - no diff",
		ExitCode: 0,
	},
	ShowResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform show success - no diff",
		ExitCode: 0,
	},
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success - no diff"}`,
		ExitCode: 0,
	},
}

func TestPlan_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                     string
		directory                string
		storageParent            string
		storagePrefix            string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expPlatformClientReqs    []*platform.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
	}{
		{
			name:                     "success_with_diff",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformDiffMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusSuccess, &platform.StatusParams{HasDiff: true, Details: "terraform show success with diff", Dir: "testdata", Operation: "plan"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "CreateObject",
					Params: []any{
						"testdata/tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "success_with_no_diff",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformNoDiffMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusNoOperation, &platform.StatusParams{HasDiff: false, Dir: "testdata", Operation: "plan"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "CreateObject",
					Params: []any{
						"testdata/tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "handles_error",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform init output",
			expStderr:                "terraform init failed",
			err:                      "failed to run Guardian plan: failed to initialize: failed to run terraform init",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusFailure, &platform.StatusParams{HasDiff: false, Details: "terraform init failed", ErrorMessage: "failed to initialize: failed to run terraform init", Dir: "testdata", Operation: "plan"}},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{}
			mockPlatformClient := &platform.MockPlatform{}

			c := &PlanCommand{
				directory:                tc.directory,
				childPath:                tc.directory,
				storagePrefix:            tc.storagePrefix,
				flagOutputDir:            t.TempDir(),
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagLockTimeout:          tc.flagLockTimeout,
				terraformClient:          tc.terraformClient,
				storageClient:            mockStorageClient,
				platformClient:           mockPlatformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if diff := cmp.Diff(mockPlatformClient.Reqs, tc.expPlatformClientReqs); diff != "" {
				t.Errorf("Platform calls not as expected; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(mockStorageClient.Reqs, tc.expStorageClientReqs); diff != "" {
				t.Errorf("Storage calls not as expected; (-got,+want): %s", diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected stdout\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected stderr\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}
