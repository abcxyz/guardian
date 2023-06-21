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

	"github.com/abcxyz/guardian/pkg/drift/assets"
	"github.com/abcxyz/guardian/pkg/drift/iam"
	"github.com/abcxyz/guardian/pkg/drift/terraform"
)

func Process(ctx context.Context, organizationID int64, bucketQuery string) error {
	assetsClient, err := assets.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("error initializing assets Client: %v", err)
	}
	folders, err := assetsClient.GetHierarchyAssets(ctx, organizationID, "cloudresourcemanager.googleapis.com/Folder")
	if err != nil {
		return fmt.Errorf("Unable to get folders: %v", err)
	}
	projects, err := assetsClient.GetHierarchyAssets(ctx, organizationID, "cloudresourcemanager.googleapis.com/Project")
	if err != nil {
		return fmt.Errorf("Unable to get folders: %v", err)
	}
	buckets, err := assetsClient.GetBuckets(ctx, organizationID, bucketQuery)
	if err != nil {
		return fmt.Errorf("error fetching terraform state GCS buckets: %v", err)
	}

	gcpIAM, err := getActualGCPIAM(ctx, organizationID, folders, projects)
	if err != nil {
		return fmt.Errorf("error determining GCP IAM: %v", err)
	}
	tfIAM, err := getTerraformStateIAM(ctx, organizationID, folders, projects, buckets)
	if err != nil {
		return fmt.Errorf("error determining Terraform State: %v", err)
	}

	clickOpsChanges := difference(gcpIAM, tfIAM)
	missingTerraformChanges := difference(tfIAM, gcpIAM)

	fmt.Println("Found Click Ops Changes: %s", clickOpsChanges)
	fmt.Println("Found Missing Terraform Changes: %s", missingTerraformChanges)

	return nil
}

func getActualGCPIAM(
	ctx context.Context,
	organizationID int64,
	folders []assets.HierarchyNode,
	projects []assets.HierarchyNode,
) (map[string]iam.AssetIAM, error) {
	client, err := iam.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing iam Client: %v", err)
	}

	m := make(map[string]iam.AssetIAM)
	for _, f := range folders {
		fIAM, err := client.GetIAMForFolder(ctx, f, "folders")
		if err != nil {
			return nil, fmt.Errorf("Unable to get folder IAM for folder with ID '%s' and name '%d': %v", f.Name, f.ID, err)
		}
		for _, i := range fIAM {
			m[iam.CreateURI(i, organizationID)] = i
		}
	}
	for _, p := range projects {
		pIAM, err := client.GetIAMForProject(ctx, p, "projects")
		if err != nil {
			return nil, fmt.Errorf("Unable to get project IAM for project with ID '%s' and name '%d': %v", p.Name, p.ID, err)
		}
		for _, i := range pIAM {
			m[iam.CreateURI(i, organizationID)] = i
		}
	}

	return m, nil
}

func getTerraformStateIAM(
	ctx context.Context,
	organizationID int64,
	folders []assets.HierarchyNode,
	projects []assets.HierarchyNode,
	gcsBuckets []string,
) (map[string]iam.AssetIAM, error) {
	parser, err := terraform.NewParser(ctx, organizationID, folders, projects)
	if err != nil {
		return nil, fmt.Errorf("error initializing terraform parser: %v", err)
	}
	gcsURIs, err := parser.GetAllTerraformStateFileURIs(ctx, gcsBuckets)
	if err != nil {
		return nil, fmt.Errorf("error determining terraform state files: %v", err)
	}

	tIAM, err := parser.ProcessAllTerraformStates(ctx, gcsURIs)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse terraform states: %v", err)
	}

	m := make(map[string]iam.AssetIAM)
	for _, i := range tIAM {
		m[iam.CreateURI(i, organizationID)] = i
	}

	return m, nil
}

func difference(source map[string]iam.AssetIAM, target map[string]iam.AssetIAM) []string {
	found := []string{}
	for key, _ := range source {
		if _, f := target[key]; !f {
			found = append(found, key)
		}
	}
	return found
}
