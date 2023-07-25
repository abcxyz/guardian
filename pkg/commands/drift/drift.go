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
	"sort"
	"strings"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/workerpool"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/iam"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
)

// IAMDrift represents the detected iam drift in a gcp org.
type IAMDrift struct {
	ClickOpsChanges         map[string]struct{}
	MissingTerraformChanges map[string]struct{}
}

type IAMDriftDetector struct {
	assetInventoryClient  assetinventory.AssetInventory
	iamClient             iam.IAM
	terraformParser       parser.Terraform
	organizationID        string
	maxConcurrentRequests int64
	foldersByID           map[string]*assetinventory.HierarchyNode
	projectsByID          map[string]*assetinventory.HierarchyNode
}

func NewIAMDriftDetector(ctx context.Context, organizationID string, maxConcurrentRequests int64) (*IAMDriftDetector, error) {
	assetInventoryClient, err := assetinventory.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize assets client: %w", err)
	}

	iamClient, err := iam.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iam client: %w", err)
	}

	terraformParser, err := parser.NewTerraformParser(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize terraform parser: %w", err)
	}

	foldersByID := make(map[string]*assetinventory.HierarchyNode)
	projectsByID := make(map[string]*assetinventory.HierarchyNode)

	return &IAMDriftDetector{
		assetInventoryClient,
		iamClient,
		terraformParser,
		organizationID,
		maxConcurrentRequests,
		foldersByID,
		projectsByID,
	}, nil
}

// DetectDrift compares the actual GCP IAM against the IAM in your Terraform state files.
func (d *IAMDriftDetector) DetectDrift(
	ctx context.Context,
	bucketQuery string,
	driftignoreFile string,
) (*IAMDrift, error) {
	logger := logging.FromContext(ctx)
	w := workerpool.New[*workerpool.Void](&workerpool.Config{
		Concurrency: d.maxConcurrentRequests,
		StopOnError: true,
	})
	// We differentiate between projects and folders here since we use them separately in
	// downstream operations.
	var folders []*assetinventory.HierarchyNode
	var projects []*assetinventory.HierarchyNode
	var buckets []string
	var err error
	if err := w.Do(ctx, func() (*workerpool.Void, error) {
		folders, err = d.assetInventoryClient.HierarchyAssets(ctx, d.organizationID, assetinventory.FolderAssetType)
		if err != nil {
			return nil, fmt.Errorf("failed to get folders: %w", err)
		}
		return nil, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to execute folder list task: %w", err)
	}
	if err := w.Do(ctx, func() (*workerpool.Void, error) {
		projects, err = d.assetInventoryClient.HierarchyAssets(ctx, d.organizationID, assetinventory.ProjectAssetType)
		if err != nil {
			return nil, fmt.Errorf("failed to get projects: %w", err)
		}
		return nil, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to execute project list task: %w", err)
	}
	if err := w.Do(ctx, func() (*workerpool.Void, error) {
		buckets, err = d.assetInventoryClient.Buckets(ctx, d.organizationID, bucketQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to determine terraform state GCS buckets: %w", err)
		}
		return nil, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to execute gcs bucket list task: %w", err)
	}
	if _, err := w.Done(ctx); err != nil {
		return nil, fmt.Errorf("failed to execute Asset tasks in parallel: %w", err)
	}
	for _, folder := range folders {
		d.foldersByID[folder.ID] = folder
	}
	for _, project := range projects {
		d.projectsByID[project.ID] = project
	}

	gcpHierarchyGraph, err := assetinventory.NewHierarchyGraph(d.organizationID, d.foldersByID, d.projectsByID)
	if err != nil {
		return nil, fmt.Errorf("failed to construct graph from GCP assets: %w", err)
	}

	ignored, err := driftignore(ctx, driftignoreFile, d.foldersByID, d.projectsByID)
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
	gcpIAM, err := d.actualGCPIAM(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine GCP IAM: %w", err)
	}
	logger.Debugw("Fetching terraform state from Buckets", "number_of_buckets", len(buckets))
	tfIAM, err := d.terraformStateIAM(ctx, buckets)
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

	clickOpsNoIgnoredChanges := DifferenceSet(clickOpsChanges, ignored.iamAssets)
	missingTerraformNoIgnoredChanges := DifferenceSet(missingTerraformChanges, ignored.iamAssets)

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
func (d *IAMDriftDetector) actualGCPIAM(ctx context.Context) (map[string]*assetinventory.AssetIAM, error) {
	w := workerpool.New[[]*assetinventory.AssetIAM](&workerpool.Config{
		Concurrency: d.maxConcurrentRequests,
		StopOnError: true,
	})
	if err := w.Do(ctx, func() ([]*assetinventory.AssetIAM, error) {
		oIAM, err := d.iamClient.OrganizationIAM(ctx, d.organizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization IAM for ID '%s': %w", d.organizationID, err)
		}
		return oIAM, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to execute org task: %w", err)
	}
	for _, f := range d.foldersByID {
		folderID := f.ID
		folderName := f.Name
		if err := w.Do(ctx, func() ([]*assetinventory.AssetIAM, error) {
			fIAM, err := d.iamClient.FolderIAM(ctx, folderID)
			if err != nil {
				return nil, fmt.Errorf("failed to get folder IAM for folder with ID '%s' and name '%s': %w", folderID, folderName, err)
			}
			return fIAM, nil
		}); err != nil {
			return nil, fmt.Errorf("failed to execute folder IAM task: %w", err)
		}
	}
	for _, pr := range d.projectsByID {
		projectID := pr.ID
		projectName := pr.Name
		if err := w.Do(ctx, func() ([]*assetinventory.AssetIAM, error) {
			pIAM, err := d.iamClient.ProjectIAM(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("failed to get project IAM for project with ID '%s' and name '%s': %w", projectID, projectName, err)
			}
			return pIAM, nil
		}); err != nil {
			return nil, fmt.Errorf("failed to execute project IAM task: %w", err)
		}
	}

	iamResults, err := w.Done(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GCP IAM tasks in parallel: %w", err)
	}

	gcpIAM := make(map[string]*assetinventory.AssetIAM)
	for _, r := range iamResults {
		if err := r.Error; err != nil {
			return nil, fmt.Errorf("failed to execute IAM task: %w", err)
		}
		for _, iamF := range r.Value {
			gcpIAM[d.URI(iamF)] = iamF
		}
	}

	return gcpIAM, nil
}

// terraformStateIAM locates all terraform state files in GCS buckets and parses them to find all IAM resources.
// Returns a map of asset URI to asset IAM.
func (d *IAMDriftDetector) terraformStateIAM(ctx context.Context, gcsBuckets []string) (map[string]*assetinventory.AssetIAM, error) {
	d.terraformParser.SetAssets(d.foldersByID, d.projectsByID)
	w := workerpool.New[[]*assetinventory.AssetIAM](&workerpool.Config{
		Concurrency: d.maxConcurrentRequests,
		StopOnError: true,
	})
	for _, b := range gcsBuckets {
		bucket := b
		if err := w.Do(ctx, func() ([]*assetinventory.AssetIAM, error) {
			gcsURIs, err := d.terraformParser.StateFileURIs(ctx, []string{bucket})
			if err != nil {
				return nil, fmt.Errorf("failed to get terraform state file URIs: %w", err)
			}
			tIAM, err := d.terraformParser.ProcessStates(ctx, gcsURIs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse terraform states: %w", err)
			}
			return tIAM, nil
		}); err != nil {
			return nil, fmt.Errorf("failed to execute terraform IAM task: %w", err)
		}
	}

	iamResults, err := w.Done(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform IAM tasks in parallel: %w", err)
	}

	tfIAM := make(map[string]*assetinventory.AssetIAM)
	for _, r := range iamResults {
		if err := r.Error; err != nil {
			return nil, fmt.Errorf("failed to execute IAM task: %w", err)
		}
		for _, iamF := range r.Value {
			tfIAM[d.URI(iamF)] = iamF
		}
	}

	return tfIAM, nil
}

// URI returns a canonical string identifier for the IAM entity.
// This is used for diffing and as output to the user.
func (d *IAMDriftDetector) URI(i *assetinventory.AssetIAM) string {
	role := strings.Replace(strings.Replace(i.Role, "organizations/", "", 1), fmt.Sprintf("%s/", d.organizationID), "", 1)
	if i.ResourceType == assetinventory.Folder {
		// Fallback to folder ID if we can not find the folder.
		resourceName := i.ResourceID
		if f, ok := d.foldersByID[i.ResourceID]; ok {
			resourceName = f.Name
		}
		return fmt.Sprintf("/organizations/%s/folders/%s/%s/%s", d.organizationID, resourceName, role, i.Member)
	} else if i.ResourceType == assetinventory.Project {
		// Fallback to project ID if we can not find the project.
		resourceName := i.ResourceID
		if p, ok := d.projectsByID[i.ResourceID]; ok {
			resourceName = p.Name
		}
		return fmt.Sprintf("/organizations/%s/projects/%s/%s/%s", d.organizationID, resourceName, role, i.Member)
	} else {
		return fmt.Sprintf("/organizations/%s/%s/%s", d.organizationID, role, i.Member)
	}
}

// differenceMap finds the keys located in the left map that are missing in the right map.
// We return a set so that we can do future comparisons easily with the result.
func differenceMap(left, right map[string]*assetinventory.AssetIAM) map[string]struct{} {
	found := make(map[string]struct{})
	for key := range left {
		if _, f := right[key]; !f {
			found[key] = struct{}{}
		}
	}
	return found
}

// DifferenceSet finds the keys located in the left set that are missing in the right set.
// We return a set so that we can do future comparisons easily with the result.
func DifferenceSet(left, right map[string]struct{}) map[string]struct{} {
	found := make(map[string]struct{})
	for key := range left {
		if _, f := right[key]; !f {
			found[key] = struct{}{}
		}
	}
	return found
}

// Keys returns the sorted keys in the map.
func Keys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
