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

package drift

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/iam"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/google/go-cmp/cmp"
)

var (
	orgID        = "1231231"
	bucket       = "my-bucket"
	bucketGCSURI = "gs://my-bucket/abcsdasd/12312/default.tfstate"
	folder       = &assetinventory.HierarchyNode{
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
	orgGroupBrowser = &iam.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "group:my-group@google.com",
		Role:         "roles/browser",
	}
	orgSABrowser = &iam.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
		Role:         "roles/browser",
	}
	orgUserBrowser = &iam.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "user:dcreey@google.com",
		Role:         "roles/browser",
	}
	folderViewer = &iam.AssetIAM{
		ResourceID:   "123123123123",
		ResourceType: "Folder",
		Member:       "group:my-group@google.com",
		Role:         "roles/viewer",
	}
	projectAdmin = &iam.AssetIAM{
		ResourceID:   "1231232222",
		ResourceType: "Project",
		Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
		Role:         "roles/compute.admin",
	}
)

func TestDrift_DetectDrift(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		terraformStatefilename string
		assetInventoryClient   assetinventory.AssetInventory
		iamClient              iam.IAM
		gcsBuckets             []string
		want                   *IAMDrift
		wantErr                string
	}{
		{
			name:                   "success_no_drift",
			terraformStatefilename: "testdata/test_valid.tfstate",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			iamClient: &iam.MockIAMClient{
				OrgData:     []*iam.AssetIAM{orgSABrowser, orgGroupBrowser, orgUserBrowser},
				FolderData:  []*iam.AssetIAM{folderViewer},
				ProjectData: []*iam.AssetIAM{projectAdmin},
			},
			want: &IAMDrift{
				ClickOpsChanges:         map[string]struct{}{},
				MissingTerraformChanges: map[string]struct{}{},
			},
		},
		{
			name:                   "success_all_click_ops_drift",
			terraformStatefilename: "testdata/test_ignored.tfstate",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			iamClient: &iam.MockIAMClient{
				OrgData:     []*iam.AssetIAM{orgSABrowser, orgGroupBrowser, orgUserBrowser},
				FolderData:  []*iam.AssetIAM{folderViewer},
				ProjectData: []*iam.AssetIAM{projectAdmin},
			},
			want: &IAMDrift{
				ClickOpsChanges: map[string]struct{}{
					"/organizations/1231231/folders/123123123123/roles/viewer/group:my-group@google.com":                                                  {},
					"/organizations/1231231/projects/my-project/roles/compute.admin/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com": {},
					"/organizations/1231231/roles/browser/group:my-group@google.com":                                                                      {},
					"/organizations/1231231/roles/browser/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com":                           {},
					"/organizations/1231231/roles/browser/user:dcreey@google.com":                                                                         {},
				},
				MissingTerraformChanges: map[string]struct{}{},
			},
		},
		{
			name:                   "success_all_missing_terraform_drift",
			terraformStatefilename: "testdata/test_valid.tfstate",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			iamClient: &iam.MockIAMClient{
				OrgData:     []*iam.AssetIAM{},
				FolderData:  []*iam.AssetIAM{},
				ProjectData: []*iam.AssetIAM{},
			},
			want: &IAMDrift{
				ClickOpsChanges: map[string]struct{}{},
				MissingTerraformChanges: map[string]struct{}{
					"/organizations/1231231/folders/123123123123/roles/viewer/group:my-group@google.com":                                                  {},
					"/organizations/1231231/projects/my-project/roles/compute.admin/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com": {},
					"/organizations/1231231/roles/browser/group:my-group@google.com":                                                                      {},
					"/organizations/1231231/roles/browser/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com":                           {},
					"/organizations/1231231/roles/browser/user:dcreey@google.com":                                                                         {},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			data, err := os.ReadFile(absolutePath(tc.terraformStatefilename))
			if err != nil {
				t.Errorf("ProcessStates() failed to read json file %v", err)
			}
			gcsClient := &storage.MockStorageClient{
				DownloadData:       string(data),
				DownloadCancelFunc: func() {},
				ListObjectURIs:     []string{bucketGCSURI},
			}
			d := &IAMDriftDetector{
				assetInventoryClient:  tc.assetInventoryClient,
				iamClient:             tc.iamClient,
				terraformParser:       &parser.TerraformParser{GCS: gcsClient, OrganizationID: orgID},
				organizationID:        orgID,
				maxConcurrentRequests: 1,
				foldersByID:           make(map[string]*assetinventory.HierarchyNode),
				projectsByID:          make(map[string]*assetinventory.HierarchyNode),
			}

			got, err := d.DetectDrift(ctx, "bucket-query", ".driftignore-not-exist")
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("DetectDrift() failed to get error %s", tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DetectDrift() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func absolutePath(filename string) (fn string) {
	_, fn, _, _ = runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(fn))))
	return fmt.Sprintf("%s/%s", repoRoot, filename)
}
