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
	"fmt"
	"log"
	"strconv"
	"strings"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	"google.golang.org/api/iterator"
	fmpb "google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	ORGANIZATION = "organization"
	FOLDER       = "folder"
	PROJECT      = "project"
)

type HierarchyNode struct {
	ID         int64
	Name       string
	ParentID   int64
	ParentType string // Either Folder or Organization
	NodeType   string // Either Folder or Organization or Project
}

type Client struct {
	assetClient *asset.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	client, err := asset.NewClient(ctx)
	if err != nil {
		log.Fatalf("asset.NewClient: %w", err)
	}
	defer client.Close()

	return &Client{
		assetClient: client,
	}, nil
}

func (c *Client) GetBuckets(ctx context.Context, organizationID int64, query string) ([]string, error) {
	// gcloud asset search-all-resources \
	// --asset-types=storage.googleapis.com/Bucket --query="$TERRAFORM_GCS_BUCKET_LABEL" --read-mask=name \
	// "--scope=organizations/$ORGANIZATION_ID"
	var readMask fmpb.FieldMask
	readMask.Paths = append(readMask.Paths, "name")
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("organizations/%d", organizationID),
		AssetTypes: []string{"storage.googleapis.com/Bucket"},
		Query:      query,
		ReadMask:   &readMask,
	}
	it := c.assetClient.SearchAllResources(ctx, req)
	results := []string{}
	for {
		resource, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error while iterating over assets %w", err)
		}
		results = append(results, resource.Name)
	}
	return results, nil
}

func (c *Client) GetHierarchyAssets(ctx context.Context, organizationID int64, assetType string) ([]HierarchyNode, error) {
	f := []HierarchyNode{}
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      fmt.Sprintf("organizations/%d", organizationID),
		AssetTypes: []string{assetType},
	}
	it := c.assetClient.SearchAllResources(ctx, req)
	for {
		resource, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error while iterating over assets %w", err)
		}
		var id int64
		assetType := strings.Replace(resource.AssetType, "cloudresourcemanager.googleapis.com/", "", 1)
		if assetType == FOLDER {
			id, err = strconv.ParseInt(strings.Replace(resource.Folders[0], "folders/", "", 1), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Unable to parse ID from folder %w: %w", resource, err)
			}
		} else if assetType == PROJECT {
			id, err = strconv.ParseInt(strings.Replace(resource.Project, "projects/", "", 1), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Unable to parse ID from folder %w: %w", resource, err)
			}
		}
		parentId, err := strconv.ParseInt(parseName(resource.ParentFullResourceName), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse parent ID from folder %w: %w", resource, err)
		}
		f = append(f, HierarchyNode{
			ID:         id,
			Name:       resource.DisplayName,
			ParentID:   parentId,
			ParentType: strings.Replace(resource.ParentAssetType, "cloudresourcemanager.googleapis.com/", "", 1),
			NodeType:   assetType,
		})
	}

	return f, nil
}

func parseName(gcpResourceName string) string {
	slice := strings.Split(gcpResourceName, "/")
	return slice[len(slice)-1]
}
