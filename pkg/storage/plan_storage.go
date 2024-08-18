// Copyright 2024 The Authors (see AUTHORS file)
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

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/platform"
)

var _ PlanStorage = (*PlanStorageClient)(nil)

// PlanStorageClient implements [PlanStorage].
type PlanStorageClient struct {
	client Storage

	storageParent string
	storagePrefix string

	GitHub github.Config
}

// PlanStorageConfig is the configuration to create a new PlanStorageClient.
type PlanStorageConfig struct {
	Platform platform.Config
}

// NewPlanStorageClient creates a new PlanStorageClient from the config.
func NewPlanStorageClient(ctx context.Context, f string, c *PlanStorageConfig) (PlanStorage, error) {
	u, err := url.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse storage flag url: %w", err)
	}

	storageType := u.Scheme

	if storageType == "" {
		storageType = c.Platform.Type
	}

	p := &PlanStorageClient{
		storageParent: path.Join(u.Host, u.Path),
	}

	if strings.EqualFold(storageType, platform.TypeLocal) {
		sc, err := NewFilesystemStorage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create file client for plan storage: %w", err)
		}

		p.client = sc
	}

	if strings.EqualFold(storageType, platform.TypeGitHub) {
		var merr error
		if c.Platform.GitHub.GitHubOwner == "" {
			merr = errors.Join(merr, fmt.Errorf("github owner is required for storage type %s", TypeGoogleCloudStorage))
		}
		if c.Platform.GitHub.GitHubRepo == "" {
			merr = errors.Join(merr, fmt.Errorf("github repo is required for storage type %s", TypeGoogleCloudStorage))
		}
		if c.Platform.GitHub.GitHubPullRequestNumber <= 0 {
			merr = errors.Join(merr, fmt.Errorf("github pull request number is required for storage type %s", TypeGoogleCloudStorage))
		}

		if merr != nil {
			return nil, merr
		}

		sc, err := NewGoogleCloudStorage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create gcs client for plan storage: %w", err)
		}

		p.storagePrefix = fmt.Sprintf("guardian-plans/%s/%s/%d", c.Platform.GitHub.GitHubOwner, c.Platform.GitHub.GitHubRepo, c.Platform.GitHub.GitHubPullRequestNumber)
		p.client = sc
	}

	return p, fmt.Errorf("unknown storage type: %s", storageType)
}

// SavePlan saves the Terraform plan file using the configured client.
func (c *PlanStorageClient) SavePlan(ctx context.Context, name string, contents []byte, metadata map[string]string) error {
	objectPath := path.Join(c.storagePrefix, name)
	if err := c.client.CreateObject(ctx, c.storageParent, objectPath, contents, WithMetadata(metadata)); err != nil {
		return fmt.Errorf("failed to save plan file: %w", err)
	}
	return nil
}

// GetPlan gets the plan file from a storage backend.
func (c *PlanStorageClient) GetPlan(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {
	objectPath := path.Join(c.storagePrefix, name)
	rc, md, err := c.client.GetObject(ctx, c.storageParent, objectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get saved plan file: %w", err)
	}
	return rc, md, nil
}

// DeletePlan deletes a plan file from storage backend.
func (c *PlanStorageClient) DeletePlan(ctx context.Context, name string) error {
	objectPath := path.Join(c.storagePrefix, name)
	if err := c.client.DeleteObject(ctx, c.storageParent, objectPath); err != nil {
		return fmt.Errorf("fail to delete saved plan file: %w", err)
	}
	return nil
}
