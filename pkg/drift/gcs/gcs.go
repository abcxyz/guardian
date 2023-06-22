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

const (
	defaultTerraformStateFileSizeLimit = 512 * 1024 * 1024 // 512 MB
)

type Client struct {
	gcs *storage.Client
}

// NewClient creates a new gcs client.
func NewClient(ctx context.Context) (*Client, error) {
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage client: %w", err)
	}

	return &Client{
		gcs: gcs,
	}, nil
}

// FilesWithName returns all files in a bucket with a given file name.
func (c *Client) FilesWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	var uris []string
	it := c.gcs.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list bucket contents: Bucket(%q).Objects(): %w", bucket, err)
		}
		if strings.HasSuffix(attrs.Name, filename) {
			uris = append(uris, fmt.Sprintf("gs://%s/%s", bucket, attrs.Name))
		}
	}
	return uris, nil
}

// DownloadAndUnmarshal fetches the file from GCS and decodes it using the
// provided unmarshaller.
//
// TODO(dcreey): Handle race conditions https://github.com/abcxyz/guardian/issues/96.
func (c *Client) DownloadAndUnmarshal(ctx context.Context, uri string, unmarshaller func(r io.Reader) error) error {
	bucketAndObject := strings.SplitN(strings.Replace(uri, "gs://", "", 1), "/", 2)
	if len(bucketAndObject) < 2 {
		return fmt.Errorf("failed to parse gcs uri: %s", uri)
	}

	bucket := bucketAndObject[0]
	object := bucketAndObject[1]

	r, err := c.gcs.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader for object %s: %w", uri, err)
	}
	defer r.Close()

	lr := io.LimitReader(r, defaultTerraformStateFileSizeLimit)
	if err := unmarshaller(lr); err != nil {
		return fmt.Errorf("failed to unmarshall gcs object %s: %w", uri, err)
	}
	return nil
}
