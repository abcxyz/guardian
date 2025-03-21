// Copyright 2024 The Authors (see AUTHORS file)
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

package cleanup

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

const (
	emptyStatefile = `{
		"version": 4,
		"terraform_version": "1.7.5",
		"serial": 1,
		"lineage": "46aca1a7-f6f7-e81e-2d38-aeb56be4640f",
		"outputs": {},
		"resources": [],
		"check_results": null
	  }
	`
	statefileWithResources = `{
		"version": 4,
		"terraform_version": "1.7.5",
		"serial": 1,
		"lineage": "46aca1a7-f6f7-e81e-2d38-aeb56be4640f",
		"outputs": {},
		"resources": [{"name": "my_resource"}],
		"check_results": null
	  }
	`
)

func TestEntrypointsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name                 string
		directory            string
		getStatefileResp     string
		isEmptyStatefile     bool
		expStorageClientReqs []*storage.Request
		err                  string
	}{
		{
			name:             "success",
			directory:        path.Join(cwd, "testdata/backends/project1"),
			getStatefileResp: emptyStatefile,
			isEmptyStatefile: true,
			expStorageClientReqs: []*storage.Request{
				{
					Name: "DeleteObject",
					Params: []any{
						"my/path/to/file/project1/default.tfstate",
					},
				},
			},
		},
		{
			name:             "success_no_delete_non_empty",
			directory:        path.Join(cwd, "testdata/backends/project2"),
			getStatefileResp: statefileWithResources,
			isEmptyStatefile: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{
				DownloadData: tc.getStatefileResp,
			}
			mockTerraformParser := &parser.MockTerraformParser{
				StateWithoutResourcesResp: tc.isEmptyStatefile,
			}

			c := &CleanupCommand{
				directory: tc.directory,

				newStorageClient: func(ctx context.Context, parent string) (storage.Storage, error) {
					return mockStorageClient, nil
				},
				terraformParser: mockTerraformParser,
			}

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
			less := func(a, b *storage.Request) bool {
				return fmt.Sprintf("%s-%s", a.Name, a.Params) < fmt.Sprintf("%s-%s", b.Name, b.Params)
			}
			if diff := cmp.Diff(mockStorageClient.Reqs, tc.expStorageClientReqs, cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("Storage calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
