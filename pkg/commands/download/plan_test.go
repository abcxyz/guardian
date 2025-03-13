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

package download

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestPlanDownload_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		directory             string
		planExitCode          string
		storageParent         string
		storagePrefix         string
		err                   string
		expPlatformClientReqs []*platform.Request
		expStorageClientReqs  []*storage.Request
		resolveJobLogsURLErr  error
	}{
		{
			name:      "success",
			directory: "testdir",

			storagePrefix: "",
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{
				Metadata: map[string]string{
					"plan_exit_code": tc.planExitCode,
				},
			}
			mockPlatformClient := &platform.MockPlatform{}

			c := &DownloadPlanCommand{
				directory:      tc.directory,
				childPath:      tc.directory,
				planFilename:   "test-tfplan.binary",
				storagePrefix:  tc.storagePrefix,
				storageClient:  mockStorageClient,
				platformClient: mockPlatformClient,
			}

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
		})
	}
}
