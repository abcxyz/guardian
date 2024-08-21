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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
)

var (
	orgID  = "1231231"
	bucket = "my-bucket"
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
	orgGroupBrowser = &assetinventory.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "group:my-group@google.com",
		Role:         "roles/browser",
	}
	orgSABrowser = &assetinventory.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
		Role:         "roles/browser",
	}
	orgUserBrowser = &assetinventory.AssetIAM{
		ResourceID:   "1231231",
		ResourceType: "Organization",
		Member:       "user:dcreey@google.com",
		Role:         "roles/browser",
	}
	folderViewer = &assetinventory.AssetIAM{
		ResourceID:   "123123123123",
		ResourceType: "Folder",
		Member:       "group:my-group@google.com",
		Role:         "roles/viewer",
	}
	projectAdmin = &assetinventory.AssetIAM{
		ResourceID:   "1231232222",
		ResourceType: "Project",
		Member:       "serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
		Role:         "roles/compute.admin",
	}
)

func TestDrift_DetectDrift(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                 string
		processStatesResp    []*assetinventory.AssetIAM
		assetInventoryClient assetinventory.AssetInventory
		gcsBuckets           []string
		want                 *IAMDrift
	}{
		{
			name: "success_no_drift",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				IAMData: []*assetinventory.AssetIAM{
					orgSABrowser,
					orgGroupBrowser,
					orgUserBrowser,
					folderViewer,
					projectAdmin,
				},
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			processStatesResp: []*assetinventory.AssetIAM{
				orgSABrowser,
				orgGroupBrowser,
				orgUserBrowser,
				folderViewer,
				projectAdmin,
			},
			want: &IAMDrift{
				ClickOpsChanges:         []string{},
				MissingTerraformChanges: []string{},
			},
		},
		{
			name: "success_all_click_ops_drift",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				IAMData: []*assetinventory.AssetIAM{
					orgSABrowser,
					orgGroupBrowser,
					orgUserBrowser,
					folderViewer,
					projectAdmin,
				},
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			want: &IAMDrift{
				ClickOpsChanges: []string{
					"/organizations/1231231/folders/123123123123/roles/viewer/group:my-group@google.com",
					"/organizations/1231231/projects/my-project/roles/compute.admin/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					"/organizations/1231231/roles/browser/group:my-group@google.com",
					"/organizations/1231231/roles/browser/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					"/organizations/1231231/roles/browser/user:dcreey@google.com",
				},
				MissingTerraformChanges: []string{},
			},
		},
		{
			name: "success_all_missing_terraform_drift",
			assetInventoryClient: &assetinventory.MockAssetInventoryClient{
				IAMData:          []*assetinventory.AssetIAM{},
				AssetFolderData:  []*assetinventory.HierarchyNode{folder},
				AssetProjectData: []*assetinventory.HierarchyNode{project},
				BucketsData:      []string{bucket},
			},
			processStatesResp: []*assetinventory.AssetIAM{
				orgSABrowser,
				orgGroupBrowser,
				orgUserBrowser,
				folderViewer,
				projectAdmin,
			},
			want: &IAMDrift{
				ClickOpsChanges: []string{},
				MissingTerraformChanges: []string{
					"/organizations/1231231/folders/123123123123/roles/viewer/group:my-group@google.com",
					"/organizations/1231231/projects/my-project/roles/compute.admin/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					"/organizations/1231231/roles/browser/group:my-group@google.com",
					"/organizations/1231231/roles/browser/serviceAccount:my-service-account@my-project.iam.gserviceaccount.com",
					"/organizations/1231231/roles/browser/user:dcreey@google.com",
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			d := &IAMDriftDetector{
				assetInventoryClient: tc.assetInventoryClient,
				terraformParser: &parser.MockTerraformParser{
					ProcessStatesResp: tc.processStatesResp,
				},
				organizationID:        orgID,
				maxConcurrentRequests: 1,
				foldersByID:           make(map[string]*assetinventory.HierarchyNode),
				projectsByID:          make(map[string]*assetinventory.HierarchyNode),
			}

			got, err := d.DetectDrift(ctx, "bucket-query", ".driftignore-not-exist")
			if err != nil {
				t.Errorf("DetectDrift() returned error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DetectDrift() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}
