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

package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Client struct {
	gcs *storage.Client
}

// NewClient creates a new gcs client.
func NewClient(ctx context.Context) (*Client, error) {
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing storage client: %w", err)
	}

	return &Client{
		gcs: gcs,
	}, nil
}

// DownloadFileIntoMemory returns all files in a bucket with a given file name.
func (c *Client) GetAllFilesWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	uris := []string{}
	it := c.gcs.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error listing bucket contents: Bucket(%q).Objects(): %w", bucket, err)
		}
		if strings.HasSuffix(attrs.Name, filename) {
			uris = append(uris, fmt.Sprintf("gs://%s/%s", bucket, attrs.Name))
		}
	}
	return uris, nil
}

// DownloadFileIntoMemory fetches the file from GCS and reads it into memory.
func (c *Client) DownloadFileIntoMemory(ctx context.Context, uri string) ([]byte, error) {
	bucketAndObject := strings.SplitN(strings.Replace(uri, "gs://", "", 1), "/", 2)

	rc, err := c.gcs.Bucket(bucketAndObject[0]).Object(bucketAndObject[1]).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("error downloading into memory: Object(%q).NewReader: %w", bucketAndObject[1], err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("error downloading into memory: ioutil.ReadAll: %w", err)
	}
	return data, nil
}
