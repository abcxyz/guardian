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

	"github.com/abcxyz/guardian/pkg/git"
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

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name                 string
		directory            string
		flagDestRef          string
		flagSourceRef        string
		gitClient            *git.MockGitClient
		getStatefileResp     string
		expStorageClientReqs []*storage.Request
		err                  string
	}{
		{
			name:             "success",
			directory:        "testdata",
			flagDestRef:      "main",
			flagSourceRef:    "ldap/feature",
			getStatefileResp: emptyStatefile,
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name:   "DownloadObject",
					Params: []any{string("my-made-up-bucket"), string("my/path/to/file/project2/default.tfstate")},
				},
				{
					Name:   "DownloadObject",
					Params: []any{string("my-made-up-bucket"), string("my/path/to/file/project1/default.tfstate")},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-made-up-bucket",
						"my/path/to/file/project1/default.tfstate",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-made-up-bucket",
						"my/path/to/file/project2/default.tfstate",
					},
				},
			},
		},
		{
			name:             "success_no_delete_non_empty",
			directory:        "testdata",
			flagDestRef:      "main",
			flagSourceRef:    "ldap/feature",
			getStatefileResp: statefileWithResources,
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name:   "DownloadObject",
					Params: []any{string("my-made-up-bucket"), string("my/path/to/file/project2/default.tfstate")},
				},
				{
					Name:   "DownloadObject",
					Params: []any{string("my-made-up-bucket"), string("my/path/to/file/project1/default.tfstate")},
				},
			},
		},
		{
			name:             "success_only_delete_changed_dir",
			directory:        "testdata",
			flagDestRef:      "main",
			flagSourceRef:    "ldap/feature",
			getStatefileResp: emptyStatefile,
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name:   "DownloadObject",
					Params: []any{string("my-made-up-bucket"), string("my/path/to/file/project1/default.tfstate")},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-made-up-bucket",
						"my/path/to/file/project1/default.tfstate",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storageClient := &storage.MockStorageClient{
				DownloadData: tc.getStatefileResp,
			}
			tfParser := &parser.TerraformParser{GCS: storageClient}

			c := &CleanupCommand{
				directory: tc.directory,

				flagDestRef:     tc.flagDestRef,
				flagSourceRef:   tc.flagSourceRef,
				gitClient:       tc.gitClient,
				storageClient:   storageClient,
				terraformParser: tfParser,
			}

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
			less := func(a, b *storage.Request) bool {
				return fmt.Sprintf("%s-%s", a.Name, a.Params) < fmt.Sprintf("%s-%s", b.Name, b.Params)
			}
			if diff := cmp.Diff(storageClient.Reqs, tc.expStorageClientReqs, cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("Storage calls not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
