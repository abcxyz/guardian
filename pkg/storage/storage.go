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

// Package storage provides an SDK for interacting with blob storage.
package storage

import (
	"context"
	"io"
)

// The types of storage clients available.
const (
	TypeFilesystem         = "file"
	TypeGoogleCloudStorage = "gcs"
)

// Storage defines the minimum interface for a blob storage system.
type Storage interface {
	// CreateObject creates a blob storage object.
	CreateObject(ctx context.Context, bucket, name string, contents []byte, opts ...CreateOption) error

	// GetObject gets a blob storage object and metadata if any. The caller must call Close on the returned Reader when done reading.
	GetObject(ctx context.Context, bucket, name string) (io.ReadCloser, map[string]string, error)

	// DeleteObject deletes a blob storage object.
	DeleteObject(ctx context.Context, bucket, name string) error

	// ObjectsWithName returns the paths of files for a given parent with the filename.
	ObjectsWithName(ctx context.Context, bucket, filename string) ([]string, error)
}

// PlanStorage defines the minimum interface for storing Terraform plan files.
type PlanStorage interface {
	// SavePlan saves a plan file to a storage backend.
	SavePlan(ctx context.Context, name string, contents []byte, metadata map[string]string) error

	// GetPlan gets the plan file from a storage backend.
	GetPlan(ctx context.Context, name string) (io.ReadCloser, map[string]string, error)

	// DeletePlan deletes a plan file from storage backend.
	DeletePlan(ctx context.Context, name string) error
}
