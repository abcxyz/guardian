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
	"fmt"
)

var _ AssetInventory = (*MockAssetInventoryClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockAssetInventoryClient struct {
	BucketsErr       string
	BucketsData      []string
	AssetFolderData  []*HierarchyNode
	AssetProjectData []*HierarchyNode
	AssetErr         string
}

func (m *MockAssetInventoryClient) Buckets(ctx context.Context, organizationID, query string) ([]string, error) {
	if m.BucketsErr != "" {
		return nil, fmt.Errorf("%s", m.BucketsErr)
	}
	return m.BucketsData, nil
}

func (m *MockAssetInventoryClient) HierarchyAssets(ctx context.Context, organizationID, assetType string) ([]*HierarchyNode, error) {
	if m.AssetErr != "" {
		return nil, fmt.Errorf("%s", m.BucketsErr)
	}
	if assetType == FolderAssetType {
		return m.AssetFolderData, nil
	}
	if assetType == ProjectAssetType {
		return m.AssetProjectData, nil
	}
	return nil, nil
}
