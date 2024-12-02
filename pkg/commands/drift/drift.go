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
	"errors"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
	"github.com/abcxyz/pkg/workerpool"
)

type TerraformStateIAMSource struct {
	*assetinventory.AssetIAM
	// The URI of the statefile.
	StateFileURI string
}

// IAMDrift represents the detected iam drift in a gcp org.
type IAMDrift struct {
	ClickOpsChanges         map[string]*assetinventory.AssetIAM
	MissingTerraformChanges map[string]*TerraformStateIAMSource
}

type IAMDriftDetector struct {
	assetInventoryClient  assetinventory.AssetInventory
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

	terraformParser, err := parser.NewTerraformParser(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize terraform parser: %w", err)
	}

	foldersByID := make(map[string]*assetinventory.HierarchyNode)
	projectsByID := make(map[string]*assetinventory.HierarchyNode)

	return &IAMDriftDetector{
		assetInventoryClient,
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

	logger.DebugContext(ctx, "fetching all iam resources for organization",
		"organization_id", d.organizationID)
	gcpIAM, err := d.actualGCPIAM(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine GCP IAM: %w", err)
	}
	logger.DebugContext(ctx, "fetching terraform state from buckets",
		"number_of_buckets", len(buckets))
	tfIAM, err := d.terraformStateIAM(ctx, buckets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IAM from Terraform State: %w", err)
	}

	gcpIAMNoIgnored := filterIgnored(gcpIAM, ignoredExpanded)
	tfIAMNoIgnored := filterIgnoredTF(tfIAM, ignoredExpanded)

	logger.DebugContext(ctx, "gcp iam entries",
		"number_of_in_scope_entries", len(gcpIAMNoIgnored),
		"number_of_entries", len(gcpIAM),
		"number_of_ignored_entries", len(gcpIAM)-len(gcpIAMNoIgnored))
	logger.DebugContext(ctx, "terraform iam entries",
		"number_of_in_scope_entries", len(tfIAMNoIgnored),
		"number_of_entries", len(tfIAM),
		"number_of_ignored_entries", len(tfIAM)-len(tfIAMNoIgnored))

	clickOpsChanges := sets.Subtract(maps.Keys(gcpIAMNoIgnored), maps.Keys(tfIAMNoIgnored))
	missingTerraformChanges := sets.Subtract(maps.Keys(tfIAMNoIgnored), maps.Keys(gcpIAMNoIgnored))

	clickOpsNoIgnoredChanges := sets.Subtract(clickOpsChanges, maps.Keys(ignored.iamAssets))
	missingTerraformNoIgnoredChanges := sets.Subtract(missingTerraformChanges, maps.Keys(ignored.iamAssets))

	clickOpsNoDefaultIgnoredChanges := filterDefaultURIs(clickOpsNoIgnoredChanges)
	missingTerraformNoDefaultIgnoredChanges := filterDefaultURIs(missingTerraformNoIgnoredChanges)

	finalClickOpsChanges := selectFrom(clickOpsNoDefaultIgnoredChanges, gcpIAM)
	finalMissingTerraformChanges := selectFrom(missingTerraformNoDefaultIgnoredChanges, tfIAM)

	logger.DebugContext(ctx, "found click ops changes",
		"number_of_in_scope_changes", len(clickOpsNoDefaultIgnoredChanges),
		"number_of_changes", len(clickOpsNoIgnoredChanges),
		"number_of_ignored_changes", (len(clickOpsChanges) - len(clickOpsNoDefaultIgnoredChanges)))
	logger.DebugContext(ctx, "found missing terraform changes",
		"number_of_in_scope_changes", len(missingTerraformNoDefaultIgnoredChanges),
		"number_of_changes", len(missingTerraformNoIgnoredChanges),
		"number_of_ignored_changes", (len(missingTerraformChanges) - len(missingTerraformNoDefaultIgnoredChanges)))

	return &IAMDrift{
		ClickOpsChanges:         finalClickOpsChanges,
		MissingTerraformChanges: finalMissingTerraformChanges,
	}, nil
}

// actualGCPIAM queries the GCP Asset Inventory to determine the IAM settings on all resources.
// Returns a map of asset URI to asset IAM.
func (d *IAMDriftDetector) actualGCPIAM(ctx context.Context) (map[string]*assetinventory.AssetIAM, error) {
	// TODO: Add query to filter out projects/folders in "DELETE_REQUESTED" state.
	iamResults, err := d.assetInventoryClient.IAM(ctx, &assetinventory.IAMOptions{
		Scope: fmt.Sprintf("organizations/%s", d.organizationID),
		AssetTypes: []string{
			"cloudresourcemanager.googleapis.com/Organization",
			"cloudresourcemanager.googleapis.com/Folder",
			"cloudresourcemanager.googleapis.com/Project",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search all IAM resources for organization: %w", err)
	}

	gcpIAM := make(map[string]*assetinventory.AssetIAM, len(iamResults))
	for _, i := range iamResults {
		gcpIAM[d.URI(i)] = i
	}

	return gcpIAM, nil
}

// terraformStateIAM locates all terraform state files in GCS buckets and parses them to find all IAM resources.
// Returns a map of asset URI to asset IAM.
func (d *IAMDriftDetector) terraformStateIAM(ctx context.Context, gcsBuckets []string) (map[string]*TerraformStateIAMSource, error) {
	d.terraformParser.SetAssets(d.foldersByID, d.projectsByID)
	w := workerpool.New[map[string][]*assetinventory.AssetIAM](&workerpool.Config{
		Concurrency: d.maxConcurrentRequests,
		StopOnError: true,
	})
	for _, b := range gcsBuckets {
		bucket := b
		if err := w.Do(ctx, func() (map[string][]*assetinventory.AssetIAM, error) {
			gcsURIs, err := d.terraformParser.StateFileURIs(ctx, []string{bucket})
			if err != nil {
				return nil, fmt.Errorf("failed to get terraform state file URIs: %w", err)
			}
			tIAM, err := d.terraformParser.ProcessStates(ctx, gcsURIs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse terraform states: %w", err)
			}
			return tIAM, nil
		}); err != nil && !errors.Is(err, workerpool.ErrStopped) {
			return nil, fmt.Errorf("failed to execute terraform IAM task: %w", err)
		}
	}

	iamResults, err := w.Done(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform IAM tasks in parallel: %w", err)
	}

	tfIAM := make(map[string]*TerraformStateIAMSource)
	errs := []error{}
	for _, r := range iamResults {
		if err := r.Error; err != nil {
			if !errors.Is(err, workerpool.ErrStopped) {
				errs = append(errs, fmt.Errorf("failed to execute IAM task: %w", err))
			}
			continue
		}
		for statefileURI, iamFs := range r.Value {
			for _, iamF := range iamFs {
				tfIAM[d.URI(iamF)] = &TerraformStateIAMSource{
					AssetIAM:     iamF,
					StateFileURI: statefileURI,
				}
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to execute terraform IAM tasks in parallel: %w", errors.Join(errs...))
	}

	return tfIAM, nil
}

// URI returns a canonical string identifier for the IAM entity.
// This is used for diffing and as output to the user.
func (d *IAMDriftDetector) URI(i *assetinventory.AssetIAM) string {
	role := strings.Replace(strings.Replace(i.Role, "organizations/", "", 1), fmt.Sprintf("%s/", d.organizationID), "", 1)
	switch i.ResourceType {
	case assetinventory.Folder:
		// Fallback to folder ID if we can not find the folder.
		resourceName := i.ResourceID
		if f, ok := d.foldersByID[i.ResourceID]; ok {
			resourceName = f.Name
		}
		return fmt.Sprintf("/organizations/%s/folders/%s/%s/%s", d.organizationID, resourceName, role, i.Member)
	case assetinventory.Project:
		// Fallback to project ID if we can not find the project.
		resourceName := i.ResourceID
		if p, ok := d.projectsByID[i.ResourceID]; ok {
			resourceName = p.Name
		}
		return fmt.Sprintf("/organizations/%s/projects/%s/%s/%s", d.organizationID, resourceName, role, i.Member)
	case assetinventory.Organization:
		return fmt.Sprintf("/organizations/%s/%s/%s", d.organizationID, role, i.Member)
	case assetinventory.Unknown:
		return fmt.Sprintf("unknownParent:/organizations/%s/%s/%s/%s/%s", d.organizationID, i.ResourceType, i.ResourceID, role, i.Member)
	default:
		return fmt.Sprintf("unknownParent:/organizations/%s/%s/%s/%s/%s", d.organizationID, i.ResourceType, i.ResourceID, role, i.Member)
	}
}

// URI returns a canonical string identifier for the IAM entity.
// This is used for diffing and as output to the user.
func ResourceURI(i *assetinventory.AssetIAM) string {
	switch i.ResourceType {
	case assetinventory.Folder:
		return fmt.Sprintf("folders/%s", i.ResourceID)
	case assetinventory.Project:
		return fmt.Sprintf("projects/%s", i.ResourceID)
	case assetinventory.Organization:
		return fmt.Sprintf("organizations/%s", i.ResourceID)
	case assetinventory.Unknown:
		return fmt.Sprintf("%s/%s", i.ResourceType, i.ResourceID)
	default:
		return fmt.Sprintf("%s/%s", i.ResourceType, i.ResourceID)
	}
}
