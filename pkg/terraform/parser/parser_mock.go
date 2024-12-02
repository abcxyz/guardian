// Copyright 2024 Google LLC
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

	"github.com/abcxyz/guardian/pkg/assetinventory"
)

// MockMockTerraformParser implements [Terraform].
type MockTerraformParser struct {
	StateFileURIsResp         []string
	StateFileURIsErr          error
	StateWithoutResourcesResp bool
	StateWithoutResourcesErr  error
	ProcessStatesResp         map[string][]*assetinventory.AssetIAM
	ProcessStatesErr          error
}

// SetAssets sets up the assets to use when looking up IAM asset bindings.
func (p *MockTerraformParser) SetAssets(
	gcpFolders map[string]*assetinventory.HierarchyNode,
	gcpProjects map[string]*assetinventory.HierarchyNode,
) {
}

// StateFileURIs finds all terraform state files in the given buckets.
func (p *MockTerraformParser) StateFileURIs(ctx context.Context, gcsBuckets []string) ([]string, error) {
	return p.StateFileURIsResp, p.StateFileURIsErr
}

// StateWithoutResources determines if the given statefile at the uri contains any resources or not.
func (p *MockTerraformParser) StateWithoutResources(ctx context.Context, uri string) (bool, error) {
	return p.StateWithoutResourcesResp, p.StateWithoutResourcesErr
}

// ProcessStates finds all IAM in memberships, bindings, or policies in the given terraform state files.
func (p *MockTerraformParser) ProcessStates(ctx context.Context, gcsUris []string) (map[string][]*assetinventory.AssetIAM, error) {
	return p.ProcessStatesResp, p.ProcessStatesErr
}
