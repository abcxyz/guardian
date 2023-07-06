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

package iam

import (
	"context"
	"fmt"

	"google.golang.org/api/cloudresourcemanager/v3"

	"github.com/abcxyz/guardian/pkg/commands/drift/assets"
)

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
}

type Client struct {
	crmService *cloudresourcemanager.Service
}

// NewClient creates a new iam client.
func NewClient(ctx context.Context) (*Client, error) {
	crm, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloudresourcemanager service: %w", err)
	}

	return &Client{
		crmService: crm,
	}, nil
}

// ProjectIAM returns all IAM memberships, bindings, and policies for the given project.
func (c *Client) ProjectIAM(ctx context.Context, projectID string) ([]*AssetIAM, error) {
	req := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			// Any operation that affects conditional role bindings must specify version `3`.
			RequestedPolicyVersion: 3,
		},
	}
	policy, err := c.crmService.Projects.GetIamPolicy(fmt.Sprintf("projects/%s", projectID), req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for project %s: %w", projectID, err)
	}
	var m []*AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   projectID,
				ResourceType: assets.Project,
			})
		}
	}

	return m, nil
}

// FolderIAM returns all IAM memberships, bindings, and policies for the given folder.
func (c *Client) FolderIAM(ctx context.Context, folderID string) ([]*AssetIAM, error) {
	req := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			// Any operation that affects conditional role bindings must specify
			// version `3`
			RequestedPolicyVersion: 3,
		},
	}
	policy, err := c.crmService.Folders.GetIamPolicy(fmt.Sprintf("folders/%s", folderID), req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for folder %s: %w", folderID, err)
	}
	var m []*AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   folderID,
				ResourceType: assets.Folder,
			})
		}
	}

	return m, nil
}

// OrganizationIAM returns all IAM memberships, bindings, and policies for the given organization.
func (c *Client) OrganizationIAM(ctx context.Context, organizationID string) ([]*AssetIAM, error) {
	req := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			// Any operation that affects conditional role bindings must specify version `3`.
			RequestedPolicyVersion: 3,
		},
	}
	policy, err := c.crmService.Projects.GetIamPolicy(fmt.Sprintf("organizations/%s", organizationID), req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for organization %s: %w", organizationID, err)
	}
	var m []*AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   organizationID,
				ResourceType: assets.Organization,
			})
		}
	}

	return m, nil
}