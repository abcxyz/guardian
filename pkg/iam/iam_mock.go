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

package iam

import (
	"context"
	"fmt"

	"github.com/abcxyz/guardian/pkg/assetinventory"
)

var _ IAM = (*MockIAMClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockIAMClient struct {
	OrgErr      string
	OrgData     []*assetinventory.AssetIAM
	FolderErr   string
	FolderData  []*assetinventory.AssetIAM
	ProjectErr  string
	ProjectData []*assetinventory.AssetIAM
}

func (m *MockIAMClient) OrganizationIAM(ctx context.Context, organizationID string) ([]*assetinventory.AssetIAM, error) {
	if m.OrgErr != "" {
		return nil, fmt.Errorf("%s", m.OrgErr)
	}
	return m.OrgData, nil
}

func (m *MockIAMClient) FolderIAM(ctx context.Context, folderID string) ([]*assetinventory.AssetIAM, error) {
	if m.FolderErr != "" {
		return nil, fmt.Errorf("%s", m.FolderErr)
	}
	return m.FolderData, nil
}

func (m *MockIAMClient) ProjectIAM(ctx context.Context, projectID string) ([]*assetinventory.AssetIAM, error) {
	if m.ProjectErr != "" {
		return nil, fmt.Errorf("%s", m.ProjectErr)
	}
	return m.ProjectData, nil
}
