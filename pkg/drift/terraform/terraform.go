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

package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/abcxyz/guardian/pkg/drift/assets"
	"github.com/abcxyz/guardian/pkg/drift/gcs"
	"github.com/abcxyz/guardian/pkg/drift/iam"
)

type ResourceInstances struct {
	Attributes struct {
		ID      string   `json:"id"`
		Members []string `json:"members,omitempty"`
	}
}

type TerraformState struct {
	Resources []struct {
		Type      string              `json:"type"`
		Instances []ResourceInstances `json:"instances"`
	} `json:"resources"`
}

type Parser struct {
	gcs             *gcs.Client
	gcpAssetsByID   map[int64]assets.HierarchyNode
	gcpAssetsByName map[string]assets.HierarchyNode
	organizationID  int64
}

func NewParser(
	ctx context.Context,
	organizationID int64,
	gcpFolders []assets.HierarchyNode,
	gcpProjects []assets.HierarchyNode,
) (*Parser, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing gcs Client: %v", err)
	}
	allAssets := append(gcpFolders, gcpProjects...)
	assetsByID := make(map[int64]assets.HierarchyNode)
	assetsByName := make(map[string]assets.HierarchyNode)
	for _, a := range allAssets {
		assetsByID[a.ID] = a
	}
	for _, a := range allAssets {
		assetsByName[a.Name] = a
	}

	return &Parser{
		gcs:             client,
		gcpAssetsByID:   assetsByID,
		gcpAssetsByName: assetsByName,
		organizationID:  organizationID,
	}, nil
}

func (p *Parser) GetAllTerraformStateFileURIs(ctx context.Context, gcsBuckets []string) ([]string, error) {
	gcsURIs := []string{}
	for _, bucket := range gcsBuckets {
		allStateFiles, err := p.gcs.GetAllFilesWithName(ctx, bucket, "default.tfstate")
		if err != nil {
			return nil, fmt.Errorf("Could not determine state files in GCS bucket %s: %v", bucket, err)
		}
		gcsURIs = append(gcsURIs, allStateFiles...)
	}
	return gcsURIs, nil
}

func (p *Parser) ProcessAllTerraformStates(ctx context.Context, gcsUris []string) ([]iam.AssetIAM, error) {
	iams := []iam.AssetIAM{}
	for _, uri := range gcsUris {
		state := TerraformState{}
		data, err := p.gcs.DownloadFileIntoMemory(ctx, uri)
		if err != nil {
			return nil, fmt.Errorf("Failed to download gcs URI for terraform: %v", err)
		}
		if err = json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("Failed to parse terraform state into json: %v", err)
		}
		iams = append(iams, p.parseTerraformStateIAM(state)...)
	}
	return iams, nil
}

func (p *Parser) parseTerraformStateIAM(state TerraformState) []iam.AssetIAM {
	iams := []iam.AssetIAM{}
	for _, r := range state.Resources {
		if strings.Contains(r.Type, "google_organization_iam_binding") {
			iams = append(iams, p.parseIAMBinding(r.Instances, true)...)
		} else if strings.Contains(r.Type, "iam_binding") {
			iams = append(iams, p.parseIAMBinding(r.Instances, false)...)
		}

		if strings.Contains(r.Type, "google_organization_iam_member") {
			iams = append(iams, p.parseIAMMember(r.Instances, true)...)
		} else if strings.Contains(r.Type, "iam_member") {
			iams = append(iams, p.parseIAMMember(r.Instances, false)...)
		}
	}
	return iams
}

func (p *Parser) parseIAMBinding(instances []ResourceInstances, isOrgLevel bool) []iam.AssetIAM {
	iams := []iam.AssetIAM{}
	for _, i := range instances {
		for _, m := range i.Attributes.Members {
			var parentID int64
			var parentType string
			if isOrgLevel {
				parentID = p.organizationID
				parentType = iam.ORGANIZATION
			} else {
				parentIDPart := ""
				parentID, parentType = p.parseParentIDAndType(parentIDPart)
			}
			role := i.Attributes.ID
			iams = append(iams, iam.AssetIAM{
				Member:     m,
				Role:       role,
				ParentID:   parentID,
				ParentType: parentType,
			})
		}
	}
	return iams
}

func (p *Parser) parseIAMMember(instances []ResourceInstances, isOrgLevel bool) []iam.AssetIAM {
	iams := []iam.AssetIAM{}
	for _, i := range instances {
		var parentID int64
		var parentType string
		if isOrgLevel {
			parentID = p.organizationID
			parentType = iam.ORGANIZATION
		} else {
			parentIDPart := ""
			parentID, parentType = p.parseParentIDAndType(parentIDPart)
		}
		member := ""
		role := i.Attributes.ID
		iams = append(iams, iam.AssetIAM{
			Member:     member,
			Role:       role,
			ParentID:   parentID,
			ParentType: parentType,
		})
	}
	return iams
}

func (p *Parser) parseParentIDAndType(parentID string) (int64, string) {
	if id, err := strconv.ParseInt(parentID, 10, 64); err == nil {
		return p.gcpAssetsByID[id].ID, p.gcpAssetsByID[id].ParentType
	} else {
		return p.gcpAssetsByName[parentID].ID, p.gcpAssetsByName[parentID].ParentType
	}
}
