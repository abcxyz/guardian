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
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/sethvargo/go-retry"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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

// IAMClient exposes GCP IAM functionality.
type IAMClient struct {
	crmService *cloudresourcemanager.Service
	cfg        *Config
}

// Config is the config values for the IAM client.
type Config struct {
	maxRetries        uint64
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
}

// NewClient creates a new iam client.
func NewClient(ctx context.Context) (*IAMClient, error) {
	crm, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloudresourcemanager service: %w", err)
	}

	cfg := &Config{
		maxRetries:        3,
		initialRetryDelay: 1 * time.Second,
		maxRetryDelay:     20 * time.Second,
	}

	return &IAMClient{
		crmService: crm,
		cfg:        cfg,
	}, nil
}

// ProjectIAM returns all IAM memberships, bindings, and policies for the given project.
func (c *IAMClient) ProjectIAM(ctx context.Context, projectID string) ([]*assetinventory.AssetIAM, error) {
	policy, err := c.projectIAMPolicy(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for project: %w", err)
	}
	return policyToAssetIAM(projectID, assetinventory.Project, policy), nil
}

// FolderIAM returns all IAM memberships, bindings, and policies for the given folder.
func (c *IAMClient) FolderIAM(ctx context.Context, folderID string) ([]*assetinventory.AssetIAM, error) {
	policy, err := c.folderIAMPolicy(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for folder: %w", err)
	}
	return policyToAssetIAM(folderID, assetinventory.Folder, policy), nil
}

// OrganizationIAM returns all IAM memberships, bindings, and policies for the given organization.
func (c *IAMClient) OrganizationIAM(ctx context.Context, organizationID string) ([]*assetinventory.AssetIAM, error) {
	policy, err := c.organizationIAMPolicy(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get iam policy for org: %w", err)
	}
	return policyToAssetIAM(organizationID, assetinventory.Organization, policy), nil
}

// RemoveProjectIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveProjectIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if err := c.withRetries(ctx, func(ctx context.Context) error {
		policy, err := c.projectIAMPolicy(ctx, iamMember.ResourceID)
		if err != nil {
			return fmt.Errorf("failed to get project policy: %w", err)
		}
		updatedPolicy := removeFromPolicy(policy, iamMember)

		req := &cloudresourcemanager.SetIamPolicyRequest{Policy: updatedPolicy}
		if _, err := c.crmService.Projects.SetIamPolicy(fmt.Sprintf("projects/%s", iamMember.ResourceID), req).Context(ctx).Do(); err != nil {
			return fmt.Errorf("failed to set project policy: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to remove project IAM: %w", err)
	}
	return nil
}

// RemoveFolderIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveFolderIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if err := c.withRetries(ctx, func(ctx context.Context) error {
		policy, err := c.folderIAMPolicy(ctx, iamMember.ResourceID)
		if err != nil {
			return fmt.Errorf("failed to get folder policy: %w", err)
		}
		updatedPolicy := removeFromPolicy(policy, iamMember)

		req := &cloudresourcemanager.SetIamPolicyRequest{Policy: updatedPolicy}
		if _, err := c.crmService.Folders.SetIamPolicy(fmt.Sprintf("folders/%s", iamMember.ResourceID), req).Context(ctx).Do(); err != nil {
			return fmt.Errorf("failed to set folder policy: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to remove folder IAM: %w", err)
	}
	return nil
}

// RemoveOrganizationIAM removes the given IAM policy membership.
func (c *IAMClient) RemoveOrganizationIAM(ctx context.Context, iamMember *assetinventory.AssetIAM) error {
	if err := c.withRetries(ctx, func(ctx context.Context) error {
		policy, err := c.organizationIAMPolicy(ctx, iamMember.ResourceID)
		if err != nil {
			return fmt.Errorf("failed to get org policy: %w", err)
		}
		updatedPolicy := removeFromPolicy(policy, iamMember)

		req := &cloudresourcemanager.SetIamPolicyRequest{Policy: updatedPolicy}
		if _, err := c.crmService.Organizations.SetIamPolicy(fmt.Sprintf("organizations/%s", iamMember.ResourceID), req).Context(ctx).Do(); err != nil {
			return fmt.Errorf("failed to set org policy: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to remove organization IAM: %w", err)
	}
	return nil
}

func (c *IAMClient) projectIAMPolicy(ctx context.Context, projectID string) (*cloudresourcemanager.Policy, error) {
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
	return policy, nil
}

func (c *IAMClient) folderIAMPolicy(ctx context.Context, folderID string) (*cloudresourcemanager.Policy, error) {
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
	return policy, nil
}

func (c *IAMClient) organizationIAMPolicy(ctx context.Context, organizationID string) (*cloudresourcemanager.Policy, error) {
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
	return policy, nil
}

func (c *IAMClient) withRetries(ctx context.Context, retryFunc retry.RetryFunc) error {
	backoff := retry.NewFibonacci(c.cfg.initialRetryDelay)
	backoff = retry.WithMaxRetries(c.cfg.maxRetries, backoff)
	backoff = retry.WithCappedDuration(c.cfg.maxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, retryFunc); err != nil {
		// IAM gRPC returns 10 on conflicts
		if terr, ok := grpcstatus.FromError(err); ok && terr.Code() == grpccodes.Aborted {
			return retry.RetryableError(err)
		}

		// IAM returns 412 while propagating, also retry on server errors
		var terr *googleapi.Error
		if ok := errors.As(err, &terr); ok && (terr.Code == 412 || terr.Code >= 500) {
			return retry.RetryableError(err)
		}

		return fmt.Errorf("failed to execute retriable function: %w", err)
	}
	return nil
}

func assetConditionString(condition assetinventory.IAMCondition) string {
	parts := []string{fmt.Sprintf("expression=%s", condition.Expression)}
	if condition.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%s", condition.Title))
	}
	if condition.Description != "" {
		parts = append(parts, fmt.Sprintf("description=%s", condition.Description))
	}
	return strings.Join(parts, ",")
}

func crmConditionString(condition cloudresourcemanager.Expr) string {
	parts := []string{fmt.Sprintf("expression=%s", condition.Expression)}
	if condition.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%s", condition.Title))
	}
	if condition.Description != "" {
		parts = append(parts, fmt.Sprintf("description=%s", condition.Description))
	}
	return strings.Join(parts, ",")
}

func policyToAssetIAM(resourceID, resourceType string, policy *cloudresourcemanager.Policy) []*assetinventory.AssetIAM {
	var m []*assetinventory.AssetIAM
	for _, b := range policy.Bindings {
		for _, member := range b.Members {
			m = append(m, &assetinventory.AssetIAM{
				Role:         b.Role,
				Member:       member,
				ResourceID:   resourceID,
				ResourceType: resourceType,
			})
		}
	}
	return m
}

func removeFromPolicy(policy *cloudresourcemanager.Policy, iam *assetinventory.AssetIAM) *cloudresourcemanager.Policy {
	var bindings []*cloudresourcemanager.Binding
	for _, b := range policy.Bindings {
		conditionsBothNil := iam.Condition == nil && b.Condition == nil
		conditionsBothFound := iam.Condition != nil && b.Condition != nil
		conditionMatch := conditionsBothNil || (conditionsBothFound && assetConditionString(*iam.Condition) == crmConditionString(*b.Condition))
		if b.Role == iam.Role && slices.Contains(b.Members, iam.Member) && conditionMatch {
			if len(b.Members) != 1 {
				var members []string
				for _, m := range b.Members {
					if m != iam.Member {
						members = append(members, m)
					}
				}
				binding := *b
				binding.Members = members
				bindings = append(bindings, &binding)
			}
		} else {
			bindings = append(bindings, b)
		}
	}
	p := *policy
	p.Bindings = bindings
	return &p
}
