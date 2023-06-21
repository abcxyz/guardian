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
	"strings"

	"google.golang.org/api/cloudresourcemanager/v3"

	"github.com/abcxyz/guardian/pkg/drift/assets"
)

const (
	ORGANIZATION = "organization"
	FOLDER       = "folder"
	PROJECT      = "project"
)

type AssetIAM struct {
	ParentID   int64
	ParentType string
	Member     string
	Role       string
}

type Client struct {
	crmService *cloudresourcemanager.Service
}

// NewClient creates a new iam client.
func NewClient(ctx context.Context) (*Client, error) {
	crm, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing cloudresourcemanager service: %w", err)
	}

	return &Client{
		crmService: crm,
	}, nil
}

// GetIAMForProject returns all IAM memberships, bindings, and policies for the given project.
func (c *Client) GetIAMForProject(ctx context.Context, node assets.HierarchyNode, api string) ([]AssetIAM, error) {
	request := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			// Any operation that affects conditional role bindings must specify
			// version `3`
			RequestedPolicyVersion: 3,
		},
	}
	policy, err := c.crmService.Projects.GetIamPolicy(fmt.Sprintf("projects/%d", node.ID), request).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get iam policy: %d: %w", node.ID, err)
	}
	m := []AssetIAM{}
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, AssetIAM{
				Role:       b.Role,
				Member:     member,
				ParentID:   node.ID,
				ParentType: node.NodeType,
			})
		}
	}

	return m, nil
}

// GetIAMForProject returns all IAM memberships, bindings, and policies for the given folder.
func (c *Client) GetIAMForFolder(ctx context.Context, node assets.HierarchyNode, api string) ([]AssetIAM, error) {
	request := &cloudresourcemanager.GetIamPolicyRequest{
		Options: &cloudresourcemanager.GetPolicyOptions{
			// Any operation that affects conditional role bindings must specify
			// version `3`
			RequestedPolicyVersion: 3,
		},
	}
	policy, err := c.crmService.Folders.GetIamPolicy(fmt.Sprintf("folders/%d", node.ID), request).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get iam policy: %d: %w", node.ID, err)
	}
	m := []AssetIAM{}
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, AssetIAM{
				Role:       b.Role,
				Member:     member,
				ParentID:   node.ID,
				ParentType: node.NodeType,
			})
		}
	}

	return m, nil
}

// CreateURI returns a canonical string identifier for the IAM entity.
func CreateURI(i AssetIAM, organizationID int64) string {
	role := strings.Replace(strings.Replace(i.Role, "organizations/", "", 1), fmt.Sprintf("%d/", organizationID), "", 1)
	if i.ParentType == FOLDER {
		return fmt.Sprintf("/organizations/%d/folders/%d/role/%s/%s", organizationID, i.ParentID, role, i.Member)
	} else if i.ParentType == PROJECT {
		return fmt.Sprintf("/organizations/%d/projects/%d/role/%s/%s", organizationID, i.ParentID, role, i.Member)
	} else {
		return fmt.Sprintf("/organizations/%d/role/%s/%s", organizationID, role, i.Member)
	}
}
