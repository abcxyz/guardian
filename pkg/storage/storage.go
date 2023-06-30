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
)

// Storage defines the minimum interface for a blob storage system.
type Storage interface {
	UploadObject(ctx context.Context, bucket, name string, contents []byte, contentType string, metadata map[string]string) error
	GetObject(ctx context.Context, bucket, name string) ([]byte, error)
	GetObjectWithLimit(ctx context.Context, bucket, name string, limit int64) ([]byte, error)
	GetMetadata(ctx context.Context, bucket, name string) (map[string]string, error)
	DeleteObject(ctx context.Context, bucket, name string, ignoreNotFound bool) error
}
