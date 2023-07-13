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

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

var _ Storage = (*MockStorageClient)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockStorageClient struct {
	reqMu sync.Mutex
	Reqs  []*Request

	UploadErr      string
	GetData        string
	GetErr         string
	GetLimitData   string
	GetLimitErr    string
	Metadata       map[string]string
	MetadataErr    string
	DeleteErr      string
	ListObjectURIs []string
	ListObjectErr  string
}

type BufferReadCloser struct {
	*bytes.Buffer
}

func (b *BufferReadCloser) Close() error { return nil }

func (m *MockStorageClient) UploadObject(ctx context.Context, bucket, name string, contents []byte, opts ...UploadOption) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "UploadObject",
		Params: []any{bucket, name, contents, opts},
	})

	if m.UploadErr != "" {
		return fmt.Errorf("%s", m.UploadErr)
	}
	return nil
}

func (m *MockStorageClient) DownloadObject(ctx context.Context, bucket, name string) (io.ReadCloser, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "DownloadObject",
		Params: []any{bucket, name},
	})

	if m.GetErr != "" {
		return nil, fmt.Errorf("%s", m.GetErr)
	}
	return &BufferReadCloser{bytes.NewBufferString(m.GetData)}, nil
}

func (m *MockStorageClient) ObjectMetadata(ctx context.Context, bucket, name string) (map[string]string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "ObjectMetadata",
		Params: []any{bucket, name},
	})

	if m.MetadataErr != "" {
		return nil, fmt.Errorf("%s", m.MetadataErr)
	}

	metadata := make(map[string]string, 0)
	if m.Metadata != nil {
		metadata = m.Metadata
	}

	return metadata, nil
}

func (m *MockStorageClient) DeleteObject(ctx context.Context, bucket, name string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "UploadObject",
		Params: []any{bucket, name},
	})

	if m.DeleteErr != "" {
		return fmt.Errorf("%s", m.DeleteErr)
	}
	return nil
}

func (m *MockStorageClient) ObjectsWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	if m.ListObjectErr != "" {
		return nil, fmt.Errorf("%s", m.ListObjectErr)
	}
	return m.ListObjectURIs, nil
}
