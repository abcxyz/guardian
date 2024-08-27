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

package apply

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

var terraformMock = &terraform.MockTerraformClient{
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	ApplyResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform apply success",
		ExitCode: 0,
	},
}

var terraformErrorMock = &terraform.MockTerraformClient{
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	ApplyResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform apply output",
		Stderr:   "terraform apply failed",
		ExitCode: 1,
		Err:      fmt.Errorf("failed to run terraform apply"),
	},
}

func TestApply_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                     string
		directory                string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		flagDestroy              bool
		planExitCode             string
		storageParent            string
		storagePrefix            string
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expReporterClientReqs    []*reporter.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
		resolveJobLogsURLErr     error
	}{
		{
			name:      "success",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "2",
			terraformClient:          terraformMock,
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusSuccess, &reporter.StatusParams{HasDiff: true, Details: "terraform apply success", Dir: "testdir", Operation: "apply"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:      "success_destroy",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagDestroy:              true,
			planExitCode:             "2",
			terraformClient:          terraformMock,
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusSuccess, &reporter.StatusParams{HasDiff: true, Details: "terraform apply success", Dir: "testdir", Operation: "apply (destroy)"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:      "skips_no_diff",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "0",
			terraformClient:          terraformMock,
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
			expStdout: "Guardian plan file has no diff, exiting",
		},
		{
			name:      "handles_error",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "2",
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform apply output",
			expStderr:                "terraform apply failed",
			err:                      "failed to run Guardian apply: failed to apply: failed to run terraform apply",
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusFailure, &reporter.StatusParams{HasDiff: true, Details: "terraform apply failed", Dir: "testdir", Operation: "apply"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:      "handles_error_destroy",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagDestroy:              true,
			planExitCode:             "2",
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform apply output",
			expStderr:                "terraform apply failed",
			err:                      "failed to run Guardian apply: failed to apply: failed to run terraform apply",
			expReporterClientReqs: []*reporter.Request{
				{
					Name:   "Status",
					Params: []any{reporter.StatusFailure, &reporter.StatusParams{HasDiff: true, Details: "terraform apply failed", Dir: "testdir", Operation: "apply (destroy)"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{
				Metadata: map[string]string{
					"plan_exit_code": tc.planExitCode,
				},
			}
			mockReporterClient := &reporter.MockReporter{}

			c := &ApplyCommand{
				directory:                tc.directory,
				childPath:                tc.directory,
				planFilename:             "test-tfplan.binary",
				storagePrefix:            tc.storagePrefix,
				flagDestroy:              tc.flagDestroy,
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagLockTimeout:          tc.flagLockTimeout,
				storageClient:            mockStorageClient,
				terraformClient:          tc.terraformClient,
				reporterClient:           mockReporterClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(mockReporterClient.Reqs, tc.expReporterClientReqs); diff != "" {
				t.Errorf("Reporter calls not as expected; (-got,+want): %s", diff)
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
