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
	"bytes"
	"context"
	"fmt"
	"io"
)

// var _ GCS = (*MockStorageClient)(nil)

type MockStorageClient struct {
	FilesURIs     []string
	FilesErr      string
	DownloadBytes []byte
	DownloadErr   string
}

func (m *MockStorageClient) FilesWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	if m.FilesErr != "" {
		return nil, fmt.Errorf("%s", m.FilesErr)
	}
	return m.FilesURIs, nil
}

func (m *MockStorageClient) DownloadAndUnmarshal(ctx context.Context, uri string, unmarshaller func(r io.Reader) error) error {
	if m.DownloadErr != "" {
		return fmt.Errorf("%s", m.DownloadErr)
	}
	return unmarshaller(bytes.NewReader(m.DownloadBytes))
}
