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
	"fmt"
	"io/ioutil"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Client struct {
	gcs *storage.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing storage client: %w", err)
	}

	return &Client{
		gcs: gcs,
	}, nil
}

func (c *Client) GetAllFilesWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	uris := []string{}
	it := c.gcs.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Bucket(%q).Objects(): %w", bucket, err)
		}
		uris = append(uris, attrs.Name)
	}
	return uris, nil
}

func (c *Client) DownloadFileIntoMemory(ctx context.Context, uri string) ([]byte, error) {
	bucketAndObject := strings.SplitN(strings.Replace(uri, "gs://", "", 1), "/", 1)

	rc, err := c.gcs.Bucket(bucketAndObject[0]).Object(bucketAndObject[1]).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("Object(%q).NewReader: %w", bucketAndObject[1], err)
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll: %w", err)
	}
	return data, nil
}
