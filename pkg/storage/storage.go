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
	"strings"
)

// Storage defines the minimum interface for a blob storage system.
type Storage interface {
	// UploadObject uploads a blob storage object.
	UploadObject(ctx context.Context, bucket, name string, contents []byte, opts ...UploadOption) error

	// DownloadObject downloads a blob storage object. The caller must call Close on the returned Reader when done reading.
	DownloadObject(ctx context.Context, bucket, name string) (io.ReadCloser, context.CancelFunc, error)

	// ObjectMetadata gets metadata for a blob storage object.
	ObjectMetadata(ctx context.Context, bucket, name string) (map[string]string, error)

	// DeleteObject deletes a blob storage object.
	DeleteObject(ctx context.Context, bucket, name string) error

	// ObjectsWithName returns the URIs of files in a given bucket with the filename.
	ObjectsWithName(ctx context.Context, bucket, filename string) ([]string, error)
}

func SplitObjectURI(uri string) (*string, *string, error) {
	bucketAndObject := strings.SplitN(strings.Replace(uri, "gs://", "", 1), "/", 2)
	if len(bucketAndObject) < 2 {
		return nil, nil, fmt.Errorf("failed to parse gcs uri: %s", uri)
	}

	return &bucketAndObject[0], &bucketAndObject[1], nil
}
