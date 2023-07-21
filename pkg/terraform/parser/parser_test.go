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

package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/google/go-cmp/cmp"
)

func TestParser_StateFileURIs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		gcsClient  storage.Storage
		gcsBuckets []string
		want       []string
		wantErr    string
	}{
		{
			name: "success",
			gcsClient: &storage.MockStorageClient{
				ListObjectURIs: []string{
					"gs://my-bucket-123/abcsdasd/12312/default.tfstate",
					"gs://my-bucket-123/abcsdasd/12313/default.tfstate",
				},
			},
			want: []string{
				"gs://my-bucket-123/abcsdasd/12312/default.tfstate",
				"gs://my-bucket-123/abcsdasd/12313/default.tfstate",
			},
			gcsBuckets: []string{"my-bucket-123"},
		},
		{
			name: "failure",
			gcsClient: &storage.MockStorageClient{
				ListObjectErr: fmt.Errorf("Failed cause 404"),
			},
			gcsBuckets: []string{"my-bucket-123"},
			wantErr:    "Failed cause 404",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := &TerraformParser{GCS: tc.gcsClient}

			got, err := p.StateFileURIs(context.Background(), tc.gcsBuckets)
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("StateFileURIs() failed to get error %s", tc.wantErr)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("StateFileURIs() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

var (
	orgID  = "1231231"
	folder = &assetinventory.HierarchyNode{
		ID:         "123123123123",
		Name:       "123123123123",
		NodeType:   assetinventory.Folder,
		ParentID:   orgID,
		ParentType: assetinventory.Organization,
	}
	project = &assetinventory.HierarchyNode{
		ID:         "1231232222",
		Name:       "my-project",
		NodeType:   assetinventory.Project,
		ParentID:   folder.ID,
		ParentType: assetinventory.Folder,
	}
)

func TestParser_ProcessStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		terraformStatefilename string
		gcsURIs                []string
		want                   []*assetinventory.AssetIAM
		wantErr                string
		knownFolders           map[string]*assetinventory.HierarchyNode
		knownProjects          map[string]*assetinventory.HierarchyNode
	}{
		{
			name:                   "success",
			terraformStatefilename: "testdata/test_valid.tfstate",
			want: []*assetinventory.AssetIAM{
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "group:my-group@google.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "user:dcreey@google.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "123123123123",
					ResourceType: "Folder",
					Member:       "group:my-group@google.com",
					Role:         "roles/viewer",
				},
				{
					ResourceID:   "1231232222",
					ResourceType: "Project",
					Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					Role:         "roles/compute.admin",
				},
			},
			gcsURIs:       []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
			knownFolders:  map[string]*assetinventory.HierarchyNode{folder.ID: folder},
			knownProjects: map[string]*assetinventory.HierarchyNode{project.ID: project},
		},
		{
			name:                   "success_no_known_assets",
			terraformStatefilename: "testdata/test_valid.tfstate",
			want: []*assetinventory.AssetIAM{
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "group:my-group@google.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "1231231",
					ResourceType: "Organization",
					Member:       "user:dcreey@google.com",
					Role:         "roles/browser",
				},
				{
					ResourceID:   "UNKNOWN_PARENT_ID",
					ResourceType: "UNKNOWN_PARENT_TYPE",
					Member:       "group:my-group@google.com",
					Role:         "roles/viewer",
				},
				{
					ResourceID:   "UNKNOWN_PARENT_ID",
					ResourceType: "UNKNOWN_PARENT_TYPE",
					Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					Role:         "roles/compute.admin",
				},
			},
			gcsURIs: []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
		},
		{
			name:                   "ignores_unsupported_iam_bindings",
			terraformStatefilename: "testdata/test_ignored.tfstate",
			want:                   nil,
			gcsURIs:                []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
		},
		{
			name:                   "failure",
			gcsURIs:                []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
			terraformStatefilename: "testdata/test_valid.tfstate",
			wantErr:                "Failed cause 404",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(absolutePath(tc.terraformStatefilename))
			if err != nil {
				t.Errorf("ProcessStates() failed to read json file %v", err)
			}

			gcsClient := &storage.MockStorageClient{
				DownloadData: string(data),
				DownloadErr:  fmt.Errorf(tc.wantErr),
			}
			p := &TerraformParser{GCS: gcsClient, OrganizationID: orgID}

			if tc.knownFolders != nil && tc.knownProjects != nil {
				p.SetAssets(tc.knownFolders, tc.knownProjects)
			}

			got, err := p.ProcessStates(context.Background(), tc.gcsURIs)
			if tc.wantErr == "" && err != nil {
				t.Errorf("ProcessStates() got unexpected error %v", err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("ProcessStates() failed to get error %s", tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ProcessStates() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func absolutePath(filename string) string {
	_, fn, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(fn))))
	return fmt.Sprintf("%s/%s", repoRoot, filename)
}
