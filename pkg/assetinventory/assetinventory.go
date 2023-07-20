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

package assetinventory

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	"google.golang.org/api/iterator"
	fmpb "google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	// Organization Node Type.
	Organization = "Organization"
	// Folder Node Type.
	Folder = "Folder"
	// Project Node Type.
	Project = "Project"

	OrganizationAssetType = "cloudresourcemanager.googleapis.com/Organization"
	FolderAssetType       = "cloudresourcemanager.googleapis.com/Folder"
	ProjectAssetType      = "cloudresourcemanager.googleapis.com/Project"
	BucketAssetType       = "storage.googleapis.com/Bucket"
)

// resourceNameIDPattern is a Regex pattern used to parse ID from the resource ParentFullResourceName.
var resourceNameIDPattern = regexp.MustCompile(`\/\/cloudresourcemanager\.googleapis\.com\/(?:folders|organizations)\/(\d*)`)

// resourceNamePattern is a Regex pattern used to parse name from the resource Name.
var resourceNamePattern = regexp.MustCompile(`\/\/cloudresourcemanager\.googleapis\.com\/(?:folders|organizations|projects)\/(.*)`)

type IAMCondition struct {
	// The title of the IAM condition.
	Title string
	// The conditional expression describing when to apply the IAM policy.
	Expression string
	// The description of the IAM condition.
	Description string
}

// AssetIAM represents the IAM of a GCP resource (e.g binding/policy/membership of GCP Project, Folder, Org).
type AssetIAM struct {
	// The ID of the resource (e.g. Project ID, Folder ID, Org ID).
	ResourceID string
	// The type of the resource (e.g. Project, Folder, Org).
	ResourceType string
	// The IAM membership (e.g. group:my-group@google.com).
	Member string
	// The role (e.g. roles/owner).
	Role string
	// The condition set on the iam.
	Condition *IAMCondition
}

// HierarchyNode represents a node in the GCP Resource Hierarchy.
// Example: Organization, Folder, or Project.
type HierarchyNode struct {
	// The unique identifier of the GCP Organization, Folder, or Project
	// Example: 123123423423
	ID string
	// The unique string name of the Organization, Folder, or Project.
	// Example: my-project-1234
	Name string
	// The unique identifier of the Folder or Organization this Folder or Project resides in.
	ParentID string
	// The type of the parent node. Either Folder or Organization.
	ParentType string
	// The type of node. Either Folder or Organization or Project
	NodeType string
}

// HierarchyNodeWithChildren represents a node in the GCP Resource Hierarchy and all of its children.
type HierarchyNodeWithChildren struct {
	*HierarchyNode
	// ProjectIDs contains the set of all projects that are immediate children of this node.
	ProjectIDs []string
	// FolderIDs contains the set of all folders that are immediate children of this node.
	FolderIDs []string
}

// HierarchyGraph represents a complete GCP Resource Hierarchy including a single organization, all of the folders and all of the projects.
type HierarchyGraph struct {
	// IDToNodes maps parent node id (e.g. folder or organization) to their children nodes (e.g. folders or projects).
	IDToNodes map[string]*HierarchyNodeWithChildren
}

// AssetInventory defines the common gcp asset inventory functionality.
type AssetInventory interface {
	// Buckets returns the GCS Buckets matching a given query.
	Buckets(ctx context.Context, organizationID, query string) ([]string, error)
	// HierarchyAssets returns the projects or folders in a given organization.
	HierarchyAssets(ctx context.Context, organizationID, assetType string) ([]*HierarchyNode, error)
	// IAM returns all IAM that matches the given query.
	IAM(ctx context.Context, scope, query string) ([]*AssetIAM, error)
}

type AssetInventoryClient struct {
	assetClient *asset.Client
}

// NewClient creates a new asset inventory client.
func NewClient(ctx context.Context) (*AssetInventoryClient, error) {
	client, err := asset.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize asset API client: %w", err)
	}

	return &AssetInventoryClient{
		assetClient: client,
	}, nil
}

// IAM returns all IAM that matches the given query.
func (c *AssetInventoryClient) IAM(ctx context.Context, scope, query string) ([]*AssetIAM, error) {
	// gcloud asset search-all-iam-policies \
	// --query="$QUERY"
	// --scope="$SCOPE"
	req := &assetpb.SearchAllIamPoliciesRequest{
		Scope: scope,
		Query: query,
	}
	it := c.assetClient.SearchAllIamPolicies(ctx, req)
	var results []*AssetIAM
	for {
		resource, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate assets: %w", err)
		}

		var resourceID string
		var resourceType string
		if resource.Project != "" {
			resourceID = strings.TrimPrefix(resource.Project, "projects/")
			resourceType = Project
		} else if len(resource.Folders) > 0 {
			resourceID = strings.TrimPrefix(resource.Folders[0], "folders/")
			resourceType = Folder
		} else {
			resourceID = strings.TrimPrefix(resource.Organization, "organizations/")
			resourceType = Organization
		}

		for _, b := range resource.Policy.Bindings {
			for _, m := range b.Members {
				results = append(results, &AssetIAM{
					Member:       m,
					Role:         b.Role,
					ResourceID:   resourceID,
					ResourceType: resourceType,
					Condition: &IAMCondition{
						Title:       b.Condition.Title,
						Expression:  b.Condition.Expression,
						Description: b.Condition.Description,
					},
				})
			}
		}
	}
	return results, nil
}

// Buckets returns all GCS Buckets in the organization that matches the given query.
func (c *AssetInventoryClient) Buckets(ctx context.Context, organizationID, query string) ([]string, error) {
	// gcloud asset search-all-resources \
	// --asset-types=storage.googleapis.com/Bucket --query="$TERRAFORM_GCS_BUCKET_LABEL" --read-mask=name \
	// "--scope=organizations/$ORGANIZATION_ID"
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("organizations/%s", organizationID),
		AssetTypes: []string{BucketAssetType},
		Query:      query,
		ReadMask: &fmpb.FieldMask{
			Paths: []string{"name"},
		},
	}
	it := c.assetClient.SearchAllResources(ctx, req)
	var results []string
	for {
		resource, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate assets: %w", err)
		}
		results = append(results, strings.TrimPrefix(resource.Name, "//storage.googleapis.com/"))
	}
	return results, nil
}

// HierarchyAssets returns all GCP Hierarchy Nodes (Folders or Projects) for the given organization.
func (c *AssetInventoryClient) HierarchyAssets(ctx context.Context, organizationID, assetType string) ([]*HierarchyNode, error) {
	var f []*HierarchyNode
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("organizations/%s", organizationID),
		AssetTypes: []string{assetType},
		Query:      "state:ACTIVE",
	}
	it := c.assetClient.SearchAllResources(ctx, req)
	for {
		resource, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate assets: %w", err)
		}
		var id string
		// Example value: "cloudresourcemanager.googleapis.com/Folder"
		assetType := strings.TrimPrefix(resource.AssetType, "cloudresourcemanager.googleapis.com/")
		if assetType == Folder {
			// Example value: "folders/123542345234"
			id = strings.TrimPrefix(resource.Folders[0], "folders/")
		} else if assetType == Project {
			// Example value: "projects/45234234234"
			id = strings.TrimPrefix(resource.Project, "projects/")
		}
		// Example value: "//cloudresourcemanager.googleapis.com/projects/my-project-name"
		name, err := extractNameFromResourceName(resource.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse name from resource name: %w", err)
		}
		// Example value: "//cloudresourcemanager.googleapis.com/folders/234234233233"
		// Example value: "//cloudresourcemanager.googleapis.com/organizations/234234233235"
		parentID, err := extractIDFromResourceName(resource.ParentFullResourceName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ID from parent resource name: %w", err)
		}
		// Example value: "cloudresourcemanager.googleapis.com/Folder"
		parentType := strings.TrimPrefix(resource.ParentAssetType, "cloudresourcemanager.googleapis.com/")
		f = append(f, &HierarchyNode{
			ID:         id,
			Name:       *name,
			ParentID:   *parentID,
			ParentType: parentType,
			NodeType:   assetType,
		})
	}

	return f, nil
}

func extractNameFromResourceName(gcpResourceName string) (*string, error) {
	matches := resourceNamePattern.FindStringSubmatch(gcpResourceName)
	if len(matches) < 2 {
		return nil, fmt.Errorf("failed to parse name from Resource Name: %s", gcpResourceName)
	}
	return &matches[1], nil
}

func extractIDFromResourceName(gcpResourceName string) (*string, error) {
	matches := resourceNameIDPattern.FindStringSubmatch(gcpResourceName)
	if len(matches) < 2 {
		return nil, fmt.Errorf("failed to parse ID from Resource Name: %s", gcpResourceName)
	}
	return &matches[1], nil
}

// NewHierarchyGraph builds a complete gcp organization graph representation of the org, its folders, and its projects.
func NewHierarchyGraph(organizationID string, folders, projects map[string]*HierarchyNode) (*HierarchyGraph, error) {
	graph := make(map[string]*HierarchyNodeWithChildren)

	graph[organizationID] = &HierarchyNodeWithChildren{
		HierarchyNode: &HierarchyNode{
			ID:         organizationID,
			Name:       "Organization",
			NodeType:   Organization,
			ParentID:   "",
			ParentType: "",
		},
		ProjectIDs: []string{},
		FolderIDs:  []string{},
	}

	for _, folder := range folders {
		if err := addFolderToGraph(graph, folder, folders); err != nil {
			return nil, fmt.Errorf("failed to traverse folders hierarchy for folder with ID %s when creating graph: %w", folder.ID, err)
		}
	}

	for _, project := range projects {
		if _, ok := graph[project.ParentID]; !ok {
			return nil, fmt.Errorf("missing reference for %s with ID %s", strings.ToLower(project.ParentType), project.ParentID)
		}
		graph[project.ParentID].ProjectIDs = append(graph[project.ParentID].ProjectIDs, project.ID)
	}

	return &HierarchyGraph{IDToNodes: graph}, nil
}

// FoldersBeneath tranverses the hierarchy graph to find all folders that are beneath a certain folder.
func FoldersBeneath(folderID string, hierarchyGraph *HierarchyGraph) (map[string]struct{}, error) {
	foundIDs := make(map[string]struct{})
	if _, ok := hierarchyGraph.IDToNodes[folderID]; !ok {
		return nil, fmt.Errorf("missing reference for folder with ID %s", folderID)
	}
	folderIDs := hierarchyGraph.IDToNodes[folderID].FolderIDs
	for _, id := range folderIDs {
		ids, err := FoldersBeneath(id, hierarchyGraph)
		if err != nil {
			return nil, fmt.Errorf("failed to find folders Beneath folder with ID %s: %w", id, err)
		}
		foundIDs[id] = struct{}{}
		for i := range ids {
			foundIDs[i] = struct{}{}
		}
	}
	return foundIDs, nil
}

func addFolderToGraph(graph map[string]*HierarchyNodeWithChildren, folder *HierarchyNode, folders map[string]*HierarchyNode) error {
	// Already added.
	if _, ok := graph[folder.ID]; ok {
		return nil
	}

	// Need to add parent node.
	if _, ok := graph[folder.ParentID]; !ok {
		if _, ok := folders[folder.ParentID]; !ok {
			return fmt.Errorf("missing reference for folder with ID %s and Name %s", folder.ParentID, folder.Name)
		}
		if err := addFolderToGraph(graph, folders[folder.ParentID], folders); err != nil {
			return fmt.Errorf("failed to add folder %s to graph: %w", folder.ParentID, err)
		}
	}

	graph[folder.ID] = &HierarchyNodeWithChildren{
		HierarchyNode: folder,
		ProjectIDs:    []string{},
		FolderIDs:     []string{},
	}

	graph[folder.ParentID].FolderIDs = append(graph[folder.ParentID].FolderIDs, folder.ID)

	return nil
}

// AssetsByName returns a map of assets keyed by asset name.
func AssetsByName(assetsByID map[string]*HierarchyNode) map[string]*HierarchyNode {
	assetsByName := make(map[string]*HierarchyNode)
	for _, a := range assetsByID {
		assetsByName[a.Name] = a
	}
	return assetsByName
}

// Merge combines two maps of assets. In the case of collision we use the asset in assetsB.
func Merge(assetsA, assetsB map[string]*HierarchyNode) map[string]*HierarchyNode {
	assets := make(map[string]*HierarchyNode)
	for _, a := range assetsA {
		assets[a.Name] = a
	}
	for _, b := range assetsB {
		assets[b.Name] = b
	}
	return assets
}
