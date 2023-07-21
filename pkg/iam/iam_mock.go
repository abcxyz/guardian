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

	"github.com/abcxyz/guardian/pkg/assetinventory"
)

var _ IAM = (*MockIAMClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockIAMClient struct {
	OrgErr           error
	OrgData          []*assetinventory.AssetIAM
	FolderErr        error
	FolderData       []*assetinventory.AssetIAM
	ProjectErr       error
	ProjectData      []*assetinventory.AssetIAM
	RemoveOrgErr     error
	RemoveFolderErr  error
	RemoveProjectErr error
}

func (m *MockIAMClient) OrganizationIAM(ctx context.Context, organizationID string) ([]*assetinventory.AssetIAM, error) {
	if m.OrgErr != nil {
		return nil, m.OrgErr
	}
	return m.OrgData, nil
}

func (m *MockIAMClient) FolderIAM(ctx context.Context, folderID string) ([]*assetinventory.AssetIAM, error) {
	if m.FolderErr != nil {
		return nil, m.FolderErr
	}
	return m.FolderData, nil
}

func (m *MockIAMClient) ProjectIAM(ctx context.Context, projectID string) ([]*assetinventory.AssetIAM, error) {
	if m.ProjectErr != nil {
		return nil, m.ProjectErr
	}
	return m.ProjectData, nil
}

func (m *MockIAMClient) RemoveOrganizationIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if m.RemoveOrgErr != nil {
		return m.RemoveOrgErr
	}
	return nil
}

func (m *MockIAMClient) RemoveFolderIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if m.RemoveFolderErr != nil {
		return m.RemoveFolderErr
	}
	return nil
}

func (m *MockIAMClient) RemoveProjectIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if m.RemoveProjectErr != nil {
		return m.RemoveProjectErr
	}
	return nil
}
