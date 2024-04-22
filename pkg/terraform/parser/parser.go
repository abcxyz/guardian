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

package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/pkg/logging"
)

const (
	// UnknownParentID is used when we are unable to find a match for the asset parent (e.g. project, folder, org)
	// This shouldn't happen but it is theoretically possible especially if there is a race condition between
	// fetching the projects & folders and querying for terraform state.
	UnknownParentID = "UNKNOWN_PARENT_ID"

	// Default max size for a terraform statefile is 512 MB.
	defaultTerraformStateFileSizeLimit = 512 * 1024 * 1024 // 512 MB

	// noResourcesInStatefileSyntax can be used to determine if a statefile has any resources or not.
	noResourcesInStatefileSyntax = "\"resources\": [],"
)

// TerraformState represents the JSON terraform state.
type TerraformState struct {
	Resources []ResourcesState `json:"resources"`
}

// ResourcesState represents the JSON for terraform state resources.
type ResourcesState struct {
	Type      string          `json:"type"`
	Instances json.RawMessage `json:"instances"`
}

// InstancesState represents the JSON terraform state Google IAM resources.
type InstancesState struct {
	Attributes *IAMAttributes `json:"attributes"`
}

// IAMAttributes represents the JSON terraform state for Gogole IAM resources attributes.
type IAMAttributes struct {
	ID      string   `json:"id"`
	Members []string `json:"members,omitempty"`
	Member  string   `json:"member,omitempty"`
	Folder  string   `json:"folder,omitempty"`
	Project string   `json:"project,omitempty"`
	Role    string   `json:"role,omitempty"`
}

// Terraform defines the common terraform functionality.
type Terraform interface {
	// SetAssets sets the assets to use for GCP asset lookup.
	SetAssets(gcpFolders, gcpProjects map[string]*assetinventory.HierarchyNode)

	// StateFileURIs returns the URIs of terraform state files located in the given GCS buckets.
	StateFileURIs(ctx context.Context, gcsBuckets []string) ([]string, error)

	// ProcessStates returns the IAM permissions stored in the given state files.
	ProcessStates(ctx context.Context, gcsUris []string) ([]*assetinventory.AssetIAM, error)

	// StateWithoutResources determines if the given statefile at the uri contains any resources or not.
	StateWithoutResources(ctx context.Context, uri string) (bool, error)
}

type TerraformParser struct {
	GCS               storage.Storage
	OrganizationID    string
	gcpAssetsByID     map[string]*assetinventory.HierarchyNode
	gcpFoldersByName  map[string]*assetinventory.HierarchyNode
	gcpProjectsByName map[string]*assetinventory.HierarchyNode
}

// NewTerraformParser creates a new terraform parser.
func NewTerraformParser(ctx context.Context, organizationID string) (*TerraformParser, error) {
	client, err := storage.NewGoogleCloudStorage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gcs Client: %w", err)
	}
	return &TerraformParser{
		GCS:               client,
		gcpAssetsByID:     make(map[string]*assetinventory.HierarchyNode),
		gcpFoldersByName:  make(map[string]*assetinventory.HierarchyNode),
		gcpProjectsByName: make(map[string]*assetinventory.HierarchyNode),
		OrganizationID:    organizationID,
	}, nil
}

// SetAssets sets up the assets to use when looking up IAM asset bindings.
func (p *TerraformParser) SetAssets(
	gcpFolders map[string]*assetinventory.HierarchyNode,
	gcpProjects map[string]*assetinventory.HierarchyNode,
) {
	p.gcpAssetsByID = assetinventory.Merge(gcpFolders, gcpProjects)
	p.gcpFoldersByName = assetinventory.AssetsByName(gcpFolders)
	p.gcpProjectsByName = assetinventory.AssetsByName(gcpProjects)
}

// StateFileURIs finds all terraform state files in the given buckets.
func (p *TerraformParser) StateFileURIs(ctx context.Context, gcsBuckets []string) ([]string, error) {
	var gcsURIs []string
	for _, bucket := range gcsBuckets {
		allStateFiles, err := p.GCS.ObjectsWithName(ctx, bucket, "default.tfstate")
		if err != nil {
			return nil, fmt.Errorf("failed to determine state files in GCS bucket %s: %w", bucket, err)
		}
		gcsURIs = append(gcsURIs, allStateFiles...)
	}
	return gcsURIs, nil
}

// StateWithoutResources determines if the given statefile at the uri contains any resources or not.
func (p *TerraformParser) StateWithoutResources(ctx context.Context, uri string) (bool, error) {
	bucket, name, err := storage.SplitObjectURI(uri)
	if err != nil {
		return false, fmt.Errorf("failed to parse GCS URI: %w", err)
	}
	r, err := p.GCS.DownloadObject(ctx, *bucket, *name)
	if err != nil {
		return false, fmt.Errorf("failed to download gcs URI for terraform: %w", err)
	}
	defer r.Close()
	lr := io.LimitReader(r, defaultTerraformStateFileSizeLimit)
	data, err := io.ReadAll(lr)
	if err != nil {
		return false, fmt.Errorf("failed to decode terraform state: %w", err)
	}
	jsonContent := string(data)
	return strings.Contains(jsonContent, noResourcesInStatefileSyntax), nil
}

// ProcessStates finds all IAM in memberships, bindings, or policies in the given terraform state files.
func (p *TerraformParser) ProcessStates(ctx context.Context, gcsUris []string) ([]*assetinventory.AssetIAM, error) {
	var iams []*assetinventory.AssetIAM
	for _, uri := range gcsUris {
		var state TerraformState
		bucket, name, err := storage.SplitObjectURI(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to parse GCS URI: %w", err)
		}
		r, err := p.GCS.DownloadObject(ctx, *bucket, *name)
		if err != nil {
			return nil, fmt.Errorf("failed to download gcs URI for terraform: %w", err)
		}
		defer r.Close()
		lr := io.LimitReader(r, defaultTerraformStateFileSizeLimit)
		if err := json.NewDecoder(lr).Decode(&state); err != nil {
			return nil, fmt.Errorf("failed to decode terraform state: %w", err)
		}
		parsedIAM, err := p.parseTerraformStateIAM(ctx, state)
		if err != nil {
			return nil, fmt.Errorf("failed to decode terraform state: %w", err)
		}
		iams = append(iams, parsedIAM...)
	}
	return iams, nil
}

func (p *TerraformParser) parseTerraformStateIAM(ctx context.Context, state TerraformState) ([]*assetinventory.AssetIAM, error) {
	var iams []*assetinventory.AssetIAM
	for _, r := range state.Resources {
		targetResources := map[string]struct{}{
			"google_organization_iam_binding": {},
			"google_folder_iam_binding":       {},
			"google_project_iam_binding":      {},
			"google_organization_iam_member":  {},
			"google_folder_iam_member":        {},
			"google_project_iam_member":       {},
		}

		// short circuit if we dont find a type we want
		if _, ok := targetResources[r.Type]; !ok {
			continue
		}

		// we have a known and expected resource type and
		// since r.Instances is json.RawMessage (equivalent to []byte)
		// we can just unmarshal the bytes into our expected struct type
		instances := make([]*InstancesState, len(r.Instances))
		if err := json.Unmarshal(r.Instances, &instances); err != nil {
			return nil, fmt.Errorf("failed to decode terraform state: %w", err)
		}

		if strings.Contains(r.Type, "google_organization_iam_binding") {
			iams = append(iams, p.parseIAMBindingForOrg(ctx, instances)...)
		} else if strings.Contains(r.Type, "google_folder_iam_binding") {
			iams = append(iams, p.parseIAMBindingForFolder(ctx, instances)...)
		} else if strings.Contains(r.Type, "google_project_iam_binding") {
			iams = append(iams, p.parseIAMBindingForProject(ctx, instances)...)
		}

		if strings.Contains(r.Type, "google_organization_iam_member") {
			iams = append(iams, p.parseIAMMemberForOrg(ctx, instances)...)
		} else if strings.Contains(r.Type, "google_folder_iam_member") {
			iams = append(iams, p.parseIAMMemberForFolder(ctx, instances)...)
		} else if strings.Contains(r.Type, "google_project_iam_member") {
			iams = append(iams, p.parseIAMMemberForProject(ctx, instances)...)
		}
	}
	return iams, nil
}

func (p *TerraformParser) parseIAMBindingForOrg(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	var iams []*assetinventory.AssetIAM
	for _, i := range instances {
		for _, m := range i.Attributes.Members {
			iams = append(iams, &assetinventory.AssetIAM{
				Member:       m,
				Role:         i.Attributes.Role,
				ResourceID:   p.OrganizationID,
				ResourceType: assetinventory.Organization,
			})
		}
	}
	return iams
}

func (p *TerraformParser) parseIAMBindingForFolder(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	var iams []*assetinventory.AssetIAM
	for _, i := range instances {
		for _, m := range i.Attributes.Members {
			folderID := strings.TrimPrefix(i.Attributes.Folder, "folders/")
			parentID, parentType := p.maybeFindGCPAssetIDAndType(folderID)
			if parentType == assetinventory.Unknown {
				logger := logging.FromContext(ctx)
				logger.WarnContext(ctx, "failed to locate GCP folder - is this folder deleted?", "folder", folderID)
			}
			iams = append(iams, &assetinventory.AssetIAM{
				Member:       m,
				Role:         i.Attributes.Role,
				ResourceID:   parentID,
				ResourceType: parentType,
			})
		}
	}
	return iams
}

func (p *TerraformParser) parseIAMBindingForProject(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	var iams []*assetinventory.AssetIAM
	for _, i := range instances {
		for _, m := range i.Attributes.Members {
			parentID, parentType := p.maybeFindGCPAssetIDAndType(i.Attributes.Project)
			if parentType == assetinventory.Unknown {
				logger := logging.FromContext(ctx)
				logger.WarnContext(ctx, "failed to locate GCP project - is this project deleted?", "project", i.Attributes.Project)
			}
			iams = append(iams, &assetinventory.AssetIAM{
				Member:       m,
				Role:         i.Attributes.Role,
				ResourceID:   parentID,
				ResourceType: parentType,
			})
		}
	}
	return iams
}

func (p *TerraformParser) parseIAMMemberForOrg(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	iams := make([]*assetinventory.AssetIAM, len(instances))
	for x, i := range instances {
		iams[x] = &assetinventory.AssetIAM{
			Member:       i.Attributes.Member,
			Role:         i.Attributes.Role,
			ResourceID:   p.OrganizationID,
			ResourceType: assetinventory.Organization,
		}
	}
	return iams
}

func (p *TerraformParser) parseIAMMemberForFolder(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	iams := make([]*assetinventory.AssetIAM, len(instances))
	for x, i := range instances {
		folderID := strings.TrimPrefix(i.Attributes.Folder, "folders/")
		parentID, parentType := p.maybeFindGCPAssetIDAndType(folderID)
		if parentType == assetinventory.Unknown {
			logger := logging.FromContext(ctx)
			logger.WarnContext(ctx, "failed to locate GCP folder - is this folder deleted?", "folder", folderID)
		}
		iams[x] = &assetinventory.AssetIAM{
			Member:       i.Attributes.Member,
			Role:         i.Attributes.Role,
			ResourceID:   parentID,
			ResourceType: parentType,
		}
	}
	return iams
}

func (p *TerraformParser) parseIAMMemberForProject(ctx context.Context, instances []*InstancesState) []*assetinventory.AssetIAM {
	iams := make([]*assetinventory.AssetIAM, len(instances))
	for x, i := range instances {
		parentID, parentType := p.maybeFindGCPAssetIDAndType(i.Attributes.Project)
		if parentType == assetinventory.Unknown {
			logger := logging.FromContext(ctx)
			logger.WarnContext(ctx, "failed to locate GCP project - is this project deleted?", "project", i.Attributes.Project)
		}
		iams[x] = &assetinventory.AssetIAM{
			Member:       i.Attributes.Member,
			Role:         i.Attributes.Role,
			ResourceID:   parentID,
			ResourceType: parentType,
		}
	}
	return iams
}

func (p *TerraformParser) maybeFindGCPAssetIDAndType(ID string) (string, string) {
	asset := p.findGCPAsset(ID)
	if asset == nil {
		return ID, assetinventory.Unknown
	}
	return asset.ID, asset.NodeType
}

// findGCPAsset attempts to find a gcp asset match for the ID.
func (p *TerraformParser) findGCPAsset(gcpAssetID string) *assetinventory.HierarchyNode {
	if _, err := strconv.ParseInt(gcpAssetID, 10, 64); err == nil {
		if _, ok := p.gcpAssetsByID[gcpAssetID]; !ok {
			return nil
		}
		return p.gcpAssetsByID[gcpAssetID]
	} else {
		if _, ok := p.gcpFoldersByName[gcpAssetID]; ok {
			return p.gcpFoldersByName[gcpAssetID]
		} else if _, ok := p.gcpProjectsByName[gcpAssetID]; ok {
			return p.gcpProjectsByName[gcpAssetID]
		}
		return nil
	}
}
