// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package assetinventory

import (
	"context"
)

var _ AssetInventory = (*MockAssetInventoryClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockAssetInventoryClient struct {
	IAMData                 []*AssetIAM
	IAMErr                  error
	BucketsData             []string
	BucketsErr              error
	AssetFolderData         []*HierarchyNode
	AssetProjectData        []*HierarchyNode
	AssetDeletedFolderData  []*HierarchyNode
	AssetDeletedProjectData []*HierarchyNode
	AssetErr                error
}

func (m *MockAssetInventoryClient) IAM(ctx context.Context, opts *IAMOptions) ([]*AssetIAM, error) {
	if m.IAMErr != nil {
		return nil, m.IAMErr
	}
	return m.IAMData, nil
}

func (m *MockAssetInventoryClient) Buckets(ctx context.Context, organizationID, query string) ([]string, error) {
	if m.BucketsErr != nil {
		return nil, m.BucketsErr
	}
	return m.BucketsData, nil
}

func (m *MockAssetInventoryClient) HierarchyAssets(ctx context.Context, organizationID, assetType, query string) ([]*HierarchyNode, error) {
	if m.AssetErr != nil {
		return nil, m.BucketsErr
	}
	if assetType == FolderAssetType && query == QueryNil {
		return m.AssetFolderData, nil
	}
	if assetType == FolderAssetType && query == QueryNotActiveResources {
		return m.AssetDeletedFolderData, nil
	}
	if assetType == ProjectAssetType && query == QueryNil {
		return m.AssetProjectData, nil
	}
	if assetType == ProjectAssetType && query == QueryNotActiveResources {
		return m.AssetDeletedProjectData, nil
	}
	return nil, nil
}
