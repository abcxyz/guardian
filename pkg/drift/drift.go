// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"strings"

	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/pkg/drift/assets"
	"github.com/abcxyz/guardian/pkg/drift/iam"
	"github.com/abcxyz/guardian/pkg/drift/terraform"
)

// Process compares the actual GCP IAM against the IAM in your Terraform state files.
func Process(ctx context.Context, organizationID, bucketQuery string) error {
	assetsClient, err := assets.NewClient(ctx)
	logger := logging.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize assets client: %w", err)
	}
	// We differentiate between projects and folders here since we use them separately in
	// downstream operations.
	folders, err := assetsClient.HierarchyAssets(ctx, organizationID, assets.FolderAssetType)
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}
	projects, err := assetsClient.HierarchyAssets(ctx, organizationID, assets.ProjectAssetType)
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}
	buckets, err := assetsClient.Buckets(ctx, organizationID, bucketQuery)
	if err != nil {
		return fmt.Errorf("failed to determine terraform state GCS buckets: %w", err)
	}
	logger.Debugw("Fetching IAM for Org, Folders and Projects", "number_of_folders", len(folders), "number_of_projects", len(projects))

	gcpIAM, err := actualGCPIAM(ctx, organizationID, folders, projects)
	if err != nil {
		return fmt.Errorf("failed to determine GCP IAM: %w", err)
	}
	logger.Debugw("Fetching terraform state from Buckets", "number_of_buckets", len(buckets))
	tfIAM, err := terraformStateIAM(ctx, organizationID, folders, projects, buckets)
	if err != nil {
		return fmt.Errorf("failed to parse IAM from Terraform State: %w", err)
	}
	logger.Debugw("GCP IAM entries", "number_of_entries", len(gcpIAM))
	logger.Debugw("Terraform IAM entries", "number_of_entries", len(tfIAM))

	clickOpsChanges := difference(gcpIAM, tfIAM)
	missingTerraformChanges := difference(tfIAM, gcpIAM)

	logger.Debugw("Found Click Ops Changes", "number_of_changes", len(clickOpsChanges))
	logger.Debugw("Found Missing Terraform Changes", "number_of_changes", len(missingTerraformChanges))

	// Output to stdout to mimic bash script for now.
	if len(clickOpsChanges) > 0 {
		fmt.Println("Found Click Ops Changes \n>", strings.Join(clickOpsChanges, "\n> "))
	}
	if len(missingTerraformChanges) > 0 {
		fmt.Println("Found Missing Terraform Changes \n>", strings.Join(missingTerraformChanges, "\n> "))
	}

	return nil
}

// actualGCPIAM queries the GCP Asset Inventory and Resource Manager to determine the IAM settings on all resources.
// Returns a map of asset URI to asset IAM.
func actualGCPIAM(
	ctx context.Context,
	organizationID string,
	folders []*assets.HierarchyNode,
	projects []*assets.HierarchyNode,
) (map[string]*iam.AssetIAM, error) {
	client, err := iam.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iam client: %w", err)
	}

	m := make(map[string]*iam.AssetIAM)
	oIAM, err := client.OrganizationIAM(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization IAM for ID '%s': %w", organizationID, err)
	}
	for _, i := range oIAM {
		m[URI(i, organizationID)] = i
	}
	for _, f := range folders {
		fIAM, err := client.FolderIAM(ctx, f.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get folder IAM for folder with ID '%s' and name '%s': %w", f.ID, f.Name, err)
		}
		for _, i := range fIAM {
			m[URI(i, organizationID)] = i
		}
	}
	for _, p := range projects {
		pIAM, err := client.ProjectIAM(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get project IAM for project with ID '%s' and name '%s': %w", p.ID, p.Name, err)
		}
		for _, i := range pIAM {
			m[URI(i, organizationID)] = i
		}
	}

	return m, nil
}

// terraformStateIAM locates all terraform state files in GCS buckets and parses them to find all IAM resources.
// Returns a map of asset URI to asset IAM.
func terraformStateIAM(
	ctx context.Context,
	organizationID string,
	folders []*assets.HierarchyNode,
	projects []*assets.HierarchyNode,
	gcsBuckets []string,
) (map[string]*iam.AssetIAM, error) {
	parser, err := terraform.NewParser(ctx, organizationID, folders, projects)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize terraform parser: %w", err)
	}
	gcsURIs, err := parser.StateFileURIs(ctx, gcsBuckets)
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform state file URIs: %w", err)
	}

	tIAM, err := parser.ProcessStates(ctx, gcsURIs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse terraform states: %w", err)
	}

	m := make(map[string]*iam.AssetIAM)
	for _, i := range tIAM {
		m[URI(i, organizationID)] = i
	}

	return m, nil
}

// Finds the keys located in the left map that are missing in the right map.
func difference(left, right map[string]*iam.AssetIAM) []string {
	var found []string
	for key := range left {
		if _, f := right[key]; !f {
			found = append(found, key)
		}
	}
	return found
}

// URI returns a canonical string identifier for the IAM entity.
// This is used for diffing and as output to the user.
func URI(i *iam.AssetIAM, organizationID string) string {
	role := strings.Replace(strings.Replace(i.Role, "organizations/", "", 1), fmt.Sprintf("%s/", organizationID), "", 1)
	if i.ResourceType == assets.Folder {
		return fmt.Sprintf("/organizations/%s/folders/%s/%s/%s", organizationID, i.ResourceID, role, i.Member)
	} else if i.ResourceType == assets.Project {
		return fmt.Sprintf("/organizations/%s/projects/%s/%s/%s", organizationID, i.ResourceID, role, i.Member)
	} else {
		return fmt.Sprintf("/organizations/%s/%s/%s", organizationID, role, i.Member)
	}
}
