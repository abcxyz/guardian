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

package assets

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

// resourceNameIDPattern is a Regex pattern used to parse ID from ParentFullResourceName.
var resourceNameIDPattern = regexp.MustCompile(`\/\/cloudresourcemanager\.googleapis\.com\/(?:folders|organizations)\/(\d*)`)

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

type Client struct {
	assetClient *asset.Client
}

// NewClient creates a new asset client.
func NewClient(ctx context.Context) (*Client, error) {
	client, err := asset.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize asset API client: %w", err)
	}

	return &Client{
		assetClient: client,
	}, nil
}

// Buckets returns all GCS Buckets in the organization that matches the given query.
func (c *Client) Buckets(ctx context.Context, organizationID, query string) ([]string, error) {
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
func (c *Client) HierarchyAssets(ctx context.Context, organizationID, assetType string) ([]*HierarchyNode, error) {
	var f []*HierarchyNode
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("organizations/%s", organizationID),
		AssetTypes: []string{assetType},
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
			Name:       resource.DisplayName,
			ParentID:   *parentID,
			ParentType: parentType,
			NodeType:   assetType,
		})
	}

	return f, nil
}

func extractIDFromResourceName(gcpResourceName string) (*string, error) {
	matches := resourceNameIDPattern.FindStringSubmatch(gcpResourceName)
	if len(matches) == 0 {
		return nil, fmt.Errorf("failed to parse ID from Resource Name: %s", gcpResourceName)
	}
	return &matches[0], nil
}
