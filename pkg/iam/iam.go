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
	"bytes"
	"context"
	"fmt"

	"google.golang.org/api/cloudresourcemanager/v3"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/child"
	"github.com/abcxyz/pkg/logging"
)

// IAM defines the common gcp iam functionality.
type IAM interface {
	// OrganizationIAM returns the IAM set on the Organization.
	OrganizationIAM(ctx context.Context, organizationID string) ([]*assetinventory.AssetIAM, error)
	// FolderIAM returns the IAM set on the Folder.
	FolderIAM(ctx context.Context, folderID string) ([]*assetinventory.AssetIAM, error)
	// ProjectIAM returns the IAM set on the Project.
	ProjectIAM(ctx context.Context, projectID string) ([]*assetinventory.AssetIAM, error)
	// RemoveOrganizationIAM removes the given IAM policy membership.
	RemoveOrganizationIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error
	// RemoveFolderIAM removes the given IAM policy membership.
	RemoveFolderIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error
	// RemoveProjectIAM removes the given IAM policy membership.
	RemoveProjectIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error
}

type IAMClient struct {
	crmService *cloudresourcemanager.Service
	workingDir *string
}

// NewClient creates a new iam client.
func NewClient(ctx context.Context, workingDir *string) (*IAMClient, error) {
	crm, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloudresourcemanager service: %w", err)
	}

	return &IAMClient{
		crmService: crm,
		workingDir: workingDir,
	}, nil
}

// ProjectIAM returns all IAM memberships, bindings, and policies for the given project.
func (c *IAMClient) ProjectIAM(ctx context.Context, projectID string) ([]*assetinventory.AssetIAM, error) {
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
	var m []*assetinventory.AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &assetinventory.AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   projectID,
				ResourceType: assetinventory.Project,
			})
		}
	}

	return m, nil
}

// FolderIAM returns all IAM memberships, bindings, and policies for the given folder.
func (c *IAMClient) FolderIAM(ctx context.Context, folderID string) ([]*assetinventory.AssetIAM, error) {
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
	var m []*assetinventory.AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &assetinventory.AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   folderID,
				ResourceType: assetinventory.Folder,
			})
		}
	}

	return m, nil
}

// OrganizationIAM returns all IAM memberships, bindings, and policies for the given organization.
func (c *IAMClient) OrganizationIAM(ctx context.Context, organizationID string) ([]*assetinventory.AssetIAM, error) {
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
	var m []*assetinventory.AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &assetinventory.AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   organizationID,
				ResourceType: assetinventory.Organization,
			})
		}
	}

	return m, nil
}

// RemoveProjectIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveProjectIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error {
	args := []string{
		"projects", "remove-iam-policy-binding",
		projectIAMMember.ResourceID,
		fmt.Sprintf("--member=%s", projectIAMMember.Member),
		fmt.Sprintf("--role=%s", projectIAMMember.Role),
	}
	if _, err := c.gcloud(ctx, args); err != nil {
		return fmt.Errorf("failed to remove iam policy for project %s: %w", projectIAMMember.ResourceID, err)
	}

	return nil
}

// RemoveFolderIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveFolderIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error {
	args := []string{
		"folders", "remove-iam-policy-binding",
		projectIAMMember.ResourceID,
		fmt.Sprintf("--member=%s", projectIAMMember.Member),
		fmt.Sprintf("--role=%s", projectIAMMember.Role),
	}
	if _, err := c.gcloud(ctx, args); err != nil {
		return fmt.Errorf("failed to remove iam policy for folder %s: %w", projectIAMMember.ResourceID, err)
	}

	return nil
}

// RemoveOrganizationIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveOrganizationIAM(ctx context.Context, projectIAMMember *assetinventory.AssetIAM) error {
	args := []string{
		"organizations", "remove-iam-policy-binding",
		projectIAMMember.ResourceID,
		fmt.Sprintf("--member=%s", projectIAMMember.Member),
		fmt.Sprintf("--role=%s", projectIAMMember.Role),
	}
	if _, err := c.gcloud(ctx, args); err != nil {
		return fmt.Errorf("failed to remove iam policy for organization %s: %w", projectIAMMember.ResourceID, err)
	}

	return nil
}

// gcloud runs the gcloud command.
func (c *IAMClient) gcloud(ctx context.Context, args []string) (int, error) {
	logger := logging.FromContext(ctx)
	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")
	exitCode, err := child.Run(ctx, &child.RunConfig{ //nolint:wrapcheck
		Stdout:     stdout,
		Stderr:     stderr,
		WorkingDir: *c.workingDir,
		Command:    "gcloud",
		Args:       args,
	})
	logger.Debugw("gcloud command executed", "args", args, "stdout", stdout)
	if err != nil {
		return exitCode, fmt.Errorf("failed to execute gcloud command: %w, \n\n %v", err, stderr)
	}
	return exitCode, nil
}
