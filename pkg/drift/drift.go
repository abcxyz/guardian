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
	"log"

	"github.com/abcxyz/guardian/pkg/drift/assets"
	"github.com/abcxyz/guardian/pkg/drift/iam"
	"github.com/abcxyz/guardian/pkg/drift/terraform"
)

// Process compares the actual GCP IAM against the IAM in your Terraform state files.
func Process(ctx context.Context, organizationID, bucketQuery string) error {
	assetsClient, err := assets.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize assets client: %w", err)
	}
	folders, err := assetsClient.HierarchyAssets(ctx, organizationID, assets.FOLDER_ASSET_TYPE)
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}
	projects, err := assetsClient.HierarchyAssets(ctx, organizationID, assets.PROJECT_ASSET_TYPE)
	if err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}
	buckets, err := assetsClient.Buckets(ctx, organizationID, bucketQuery)
	if err != nil {
		return fmt.Errorf("failed to determine terraform state GCS buckets: %w", err)
	}
	log.Printf("Fetching IAM for 1 Org, %d Folders and %d Projects\n", len(folders), len(projects))

	gcpIAM, err := actualGCPIAM(ctx, organizationID, folders, projects)
	if err != nil {
		return fmt.Errorf("failed to determine GCP IAM: %w", err)
	}
	log.Printf("Fetching terraform state from %d Buckets\n", len(buckets))
	tfIAM, err := terraformStateIAM(ctx, organizationID, folders, projects, buckets)
	if err != nil {
		return fmt.Errorf("failed to parse IAM from Terraform State: %w", err)
	}
	log.Printf("Found %d gcp IAM entries\n", len(gcpIAM))
	log.Printf("Found %d terraform IAM entries\n", len(tfIAM))

	clickOpsChanges := difference(gcpIAM, tfIAM)
	missingTerraformChanges := difference(tfIAM, gcpIAM)

	log.Printf("Found %d Click Ops Changes\n", len(clickOpsChanges))
	log.Printf("Found %d Missing Terraform Changes\n", len(missingTerraformChanges))

	return nil
}

// actualGCPIAM queries the GCP Asset Inventory and Resource Manager to determine the IAM settings on all resources.
// Returns a map of asset URI to asset IAM.
func actualGCPIAM(
	ctx context.Context,
	organizationID string,
	folders []assets.HierarchyNode,
	projects []assets.HierarchyNode,
) (map[string]*iam.AssetIAM, error) {
	client, err := iam.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iam client: %w", err)
	}

	m := make(map[string]*iam.AssetIAM)
	oIAM, err := client.OrganizationIAM(ctx, organizationID, "folders")
	if err != nil {
		return nil, fmt.Errorf("failed to get organization IAM for ID '%s': %w", organizationID, err)
	}
	for _, i := range oIAM {
		m[iam.URI(i, organizationID)] = i
	}
	for _, f := range folders {
		fIAM, err := client.FolderIAM(ctx, f.ID, "folders")
		if err != nil {
			return nil, fmt.Errorf("failed to get folder IAM for folder with ID '%s' and name '%s': %w", f.ID, f.Name, err)
		}
		for _, i := range fIAM {
			m[iam.URI(i, organizationID)] = i
		}
	}
	for _, p := range projects {
		pIAM, err := client.ProjectIAM(ctx, p.ID, "projects")
		if err != nil {
			return nil, fmt.Errorf("failed to get project IAM for project with ID '%s' and name '%s': %w", p.ID, p.Name, err)
		}
		for _, i := range pIAM {
			m[iam.URI(i, organizationID)] = i
		}
	}

	return m, nil
}

// terraformStateIAM locates all terraform state files in GCS buckets and parses them to find all IAM resources.
// Returns a map of asset URI to asset IAM.
func terraformStateIAM(
	ctx context.Context,
	organizationID string,
	folders []assets.HierarchyNode,
	projects []assets.HierarchyNode,
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
		m[iam.URI(i, organizationID)] = i
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
