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
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
)

// The types of storage clients available.
const (
	TypeFilesystem         = "file"
	TypeGoogleCloudStorage = "gcs"
)

// SortedStorageTypes are the sorted Storage types for printing messages and prediction.
var SortedStorageTypes = func() []string {
	allowed := append([]string{}, TypeFilesystem, TypeGoogleCloudStorage)
	sort.Strings(allowed)
	return allowed
}()

// Storage defines the minimum interface for a blob storage system.
type Storage interface {
	// CreateObject creates a blob storage object.
	CreateObject(ctx context.Context, name string, contents []byte, opts ...CreateOption) error

	// GetObject gets a blob storage object and metadata if any. The caller must call Close on the returned Reader when done reading.
	GetObject(ctx context.Context, name string) (io.ReadCloser, map[string]string, error)

	// DeleteObject deletes a blob storage object.
	DeleteObject(ctx context.Context, name string) error

	// ObjectsWithName returns the paths of files for a given parent with the filename.
	ObjectsWithName(ctx context.Context, name string) ([]string, error)
}

// NewStorageClient creates a new storage client based on the provided type.
func NewStorageClient(ctx context.Context, t, parent string) (Storage, error) {
	if strings.EqualFold(t, TypeFilesystem) {
		return NewFilesystemStorage(ctx, parent)
	}

	if strings.EqualFold(t, TypeGoogleCloudStorage) {
		return NewGoogleCloudStorage(ctx, parent)
	}

	return nil, fmt.Errorf("unknown storage type: %s", t)
}

// Parse creates a new  storage client by parsing a storage url.
func Parse(ctx context.Context, u string) (Storage, error) {
	if u == "" {
		return nil, fmt.Errorf("url is requierd")
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse storage url: %w", err)
	}

	parent := path.Join(parsed.Host, parsed.Path)

	if strings.EqualFold(parsed.Scheme, TypeFilesystem) {
		return NewFilesystemStorage(ctx, parent)
	}

	if strings.EqualFold(parsed.Scheme, TypeGoogleCloudStorage) {
		return NewGoogleCloudStorage(ctx, parent)
	}

	return nil, fmt.Errorf("unknown storage type: %s", parsed.Scheme)
}
