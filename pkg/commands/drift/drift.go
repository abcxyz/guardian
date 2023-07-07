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
	"github.com/sourcegraph/conc/pool"

	"github.com/abcxyz/guardian/pkg/commands/drift/assets"
	"github.com/abcxyz/guardian/pkg/commands/drift/iam"
	"github.com/abcxyz/guardian/pkg/commands/drift/terraform"
)

const (
	maxConcurrentRequests = 10
)

// IAMDrift represents the detected iam drift in a gcp org.
type IAMDrift struct {
	ClickOpsChanges         map[string]struct{}
	MissingTerraformChanges map[string]struct{}
}

// Process compares the actual GCP IAM against the IAM in your Terraform state files.
func Process(ctx context.Context, organizationID, bucketQuery, driftignoreFile string) (*IAMDrift, error) {
	assetsClient, err := assets.NewClient(ctx)
	logger := logging.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize assets client: %w", err)
	}
	// We differentiate between projects and folders here since we use them separately in
	// downstream operations.
	p := pool.New().WithErrors()
	var folders []*assets.HierarchyNode
	var projects []*assets.HierarchyNode
	var buckets []string
	p.Go(func() error {
		folders, err = assetsClient.HierarchyAssets(ctx, organizationID, assets.FolderAssetType)
		if err != nil {
			return fmt.Errorf("failed to get folders: %w", err)
		}
		return nil
	})
	p.Go(func() error {
		projects, err = assetsClient.HierarchyAssets(ctx, organizationID, assets.ProjectAssetType)
		if err != nil {
			return fmt.Errorf("failed to get folders: %w", err)
		}
		return nil
	})
	p.Go(func() error {
		buckets, err = assetsClient.Buckets(ctx, organizationID, bucketQuery)
		if err != nil {
			return fmt.Errorf("failed to determine terraform state GCS buckets: %w", err)
		}
		return nil
	})
	if err := p.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute Asset tasks in parallel: %w", err)
	}
	foldersByID := make(map[string]*assets.HierarchyNode)
	for _, folder := range folders {
		foldersByID[folder.ID] = folder
	}
	projectsByID := make(map[string]*assets.HierarchyNode)
	for _, project := range projects {
		projectsByID[project.ID] = project
	}

	gcpHierarchyGraph, err := assets.NewHierarchyGraph(organizationID, foldersByID, projectsByID)
	if err != nil {
		return nil, fmt.Errorf("failed to construct graph from GCP assets: %w", err)
	}

	ignored, err := driftignore(ctx, driftignoreFile, foldersByID, projectsByID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse driftignore file: %w", err)
	}
	ignoredExpanded, err := expandGraph(ignored, gcpHierarchyGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to expand graph for ignored assets: %w", err)
	}

	logger.Debugw("fetching iam for org, folders and projects",
		"number_of_folders", len(folders),
		"number_of_projects", len(projects))
	gcpIAM, err := actualGCPIAM(ctx, organizationID, foldersByID, projectsByID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine GCP IAM: %w", err)
	}
	logger.Debugw("Fetching terraform state from Buckets", "number_of_buckets", len(buckets))
	tfIAM, err := terraformStateIAM(ctx, organizationID, foldersByID, projectsByID, buckets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IAM from Terraform State: %w", err)
	}

	gcpIAMNoIgnored := filterIgnored(gcpIAM, ignoredExpanded)
	tfIAMNoIgnored := filterIgnored(tfIAM, ignoredExpanded)

	logger.Debugw("gcp iam entries",
		"number_of_in_scope_entries", len(gcpIAMNoIgnored),
		"number_of_entries", len(gcpIAM),
		"number_of_ignored_entries", len(gcpIAM)-len(gcpIAMNoIgnored))
	logger.Debugw("terraform iam entries",
		"number_of_in_scope_entries", len(tfIAMNoIgnored),
		"number_of_entries", len(tfIAM),
		"number_of_ignored_entries", len(tfIAM)-len(tfIAMNoIgnored))

	clickOpsChanges := differenceMap(gcpIAMNoIgnored, tfIAMNoIgnored)
	missingTerraformChanges := differenceMap(tfIAMNoIgnored, gcpIAMNoIgnored)

	clickOpsNoIgnoredChanges := differenceSet(clickOpsChanges, ignored.iamAssets)
	missingTerraformNoIgnoredChanges := differenceSet(missingTerraformChanges, ignored.iamAssets)

	clickOpsNoDefaultIgnoredChanges := filterDefaultURIs(clickOpsNoIgnoredChanges)
	missingTerraformNoDefaultIgnoredChanges := filterDefaultURIs(missingTerraformNoIgnoredChanges)

	logger.Debugw("found click ops changes",
		"number_of_in_scope_changes", len(clickOpsNoDefaultIgnoredChanges),
		"number_of_changes", len(clickOpsNoIgnoredChanges),
		"number_of_ignored_changes", (len(clickOpsChanges) - len(clickOpsNoDefaultIgnoredChanges)))
	logger.Debugw("found missing terraform changes",
		"number_of_in_scope_changes", len(missingTerraformNoDefaultIgnoredChanges),
		"number_of_changes", len(missingTerraformNoIgnoredChanges),
		"number_of_ignored_changes", (len(missingTerraformChanges) - len(missingTerraformNoDefaultIgnoredChanges)))

	return &IAMDrift{
		ClickOpsChanges:         clickOpsNoDefaultIgnoredChanges,
		MissingTerraformChanges: missingTerraformNoDefaultIgnoredChanges,
	}, nil
}

// actualGCPIAM queries the GCP Asset Inventory and Resource Manager to determine the IAM settings on all resources.
// Returns a map of asset URI to asset IAM.
func actualGCPIAM(
	ctx context.Context,
	organizationID string,
	foldersByID map[string]*assets.HierarchyNode,
	projectsByID map[string]*assets.HierarchyNode,
) (map[string]*iam.AssetIAM, error) {
	client, err := iam.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iam client: %w", err)
	}

	p := pool.New().
		WithMaxGoroutines(maxConcurrentRequests).
		WithContext(ctx).
		WithCancelOnError()
	gcpIAMCount := 1 + len(foldersByID) + len(projectsByID)
	iamChan := make(chan []*iam.AssetIAM, gcpIAMCount)

	p.Go(func(ctx context.Context) error {
		oIAM, err := client.OrganizationIAM(ctx, organizationID)
		if err != nil {
			return fmt.Errorf("failed to get organization IAM for ID '%s': %w", organizationID, err)
		}
		iamChan <- oIAM
		return nil
	})
	for _, f := range foldersByID {
		folderID := f.ID
		folderName := f.Name
		p.Go(func(ctx context.Context) error {
			fIAM, err := client.FolderIAM(ctx, folderID)
			if err != nil {
				return fmt.Errorf("failed to get folder IAM for folder with ID '%s' and name '%s': %w", folderID, folderName, err)
			}
			iamChan <- fIAM
			return nil
		})
	}
	for _, pr := range projectsByID {
		projectID := pr.ID
		projectName := pr.Name
		p.Go(func(ctx context.Context) error {
			pIAM, err := client.ProjectIAM(ctx, projectID)
			if err != nil {
				return fmt.Errorf("failed to get project IAM for project with ID '%s' and name '%s': %w", projectID, projectName, err)
			}
			iamChan <- pIAM
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute GCP IAM tasks in parallel: %w", err)
	}

	gcpIAM := make(map[string]*iam.AssetIAM)
	for i := 0; i < gcpIAMCount; i++ {
		for _, iamF := range <-iamChan {
			gcpIAM[URI(iamF, organizationID, foldersByID, projectsByID)] = iamF
		}
	}

	return gcpIAM, nil
}

// terraformStateIAM locates all terraform state files in GCS buckets and parses them to find all IAM resources.
// Returns a map of asset URI to asset IAM.
func terraformStateIAM(
	ctx context.Context,
	organizationID string,
	foldersByID map[string]*assets.HierarchyNode,
	projectsByID map[string]*assets.HierarchyNode,
	gcsBuckets []string,
) (map[string]*iam.AssetIAM, error) {
	parser, err := terraform.NewParser(ctx, organizationID, foldersByID, projectsByID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize terraform parser: %w", err)
	}
	p := pool.New().
		WithMaxGoroutines(maxConcurrentRequests).
		WithContext(ctx).
		WithCancelOnError()
	iamChan := make(chan []*iam.AssetIAM, len(gcsBuckets))
	for _, b := range gcsBuckets {
		bucket := b
		p.Go(func(ctx context.Context) error {
			gcsURIs, err := parser.StateFileURIs(ctx, []string{bucket})
			if err != nil {
				return fmt.Errorf("failed to get terraform state file URIs: %w", err)
			}

			tIAM, err := parser.ProcessStates(ctx, gcsURIs)
			if err != nil {
				return fmt.Errorf("failed to parse terraform states: %w", err)
			}
			iamChan <- tIAM
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return nil, fmt.Errorf("failed to execute terraform IAM tasks in parallel: %w", err)
	}

	tfIAM := make(map[string]*iam.AssetIAM)
	for i := 0; i < len(gcsBuckets); i++ {
		for _, iamF := range <-iamChan {
			tfIAM[URI(iamF, organizationID, foldersByID, projectsByID)] = iamF
		}
	}

	return tfIAM, nil
}

// differenceMap finds the keys located in the left map that are missing in the right map.
// We return a set so that we can do future comparisons easily with the result.
func differenceMap(left, right map[string]*iam.AssetIAM) map[string]struct{} {
	found := make(map[string]struct{})
	for key := range left {
		if _, f := right[key]; !f {
			found[key] = struct{}{}
		}
	}
	return found
}

// differenceSet finds the keys located in the left set that are missing in the right set.
// We return a set so that we can do future comparisons easily with the result.
func differenceSet(left, right map[string]struct{}) map[string]struct{} {
	found := make(map[string]struct{})
	for key := range left {
		if _, f := right[key]; !f {
			found[key] = struct{}{}
		}
	}
	return found
}

// URI returns a canonical string identifier for the IAM entity.
// This is used for diffing and as output to the user.
func URI(
	i *iam.AssetIAM,
	organizationID string,
	foldersByID map[string]*assets.HierarchyNode,
	projectsByID map[string]*assets.HierarchyNode,
) string {
	role := strings.Replace(strings.Replace(i.Role, "organizations/", "", 1), fmt.Sprintf("%s/", organizationID), "", 1)
	if i.ResourceType == assets.Folder {
		// Fallback to folder ID if we can not find the folder.
		resourceName := i.ResourceID
		if f, ok := foldersByID[i.ResourceID]; ok {
			resourceName = f.Name
		}
		return fmt.Sprintf("/organizations/%s/folders/%s/%s/%s", organizationID, resourceName, role, i.Member)
	} else if i.ResourceType == assets.Project {
		// Fallback to project ID if we can not find the project.
		resourceName := i.ResourceID
		if p, ok := projectsByID[i.ResourceID]; ok {
			resourceName = p.Name
		}
		return fmt.Sprintf("/organizations/%s/projects/%s/%s/%s", organizationID, resourceName, role, i.Member)
	} else {
		return fmt.Sprintf("/organizations/%s/%s/%s", organizationID, role, i.Member)
	}
}
