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

	UploadErr      error
	DownloadData   string
	DownloadErr    error
	Metadata       map[string]string
	MetadataErr    error
	DeleteErr      error
	ListObjectURIs []string
	ListObjectErr  error
}

type BufferReadCloser struct {
	*bytes.Buffer
}

func (b *BufferReadCloser) Close() error { return nil }

func (m *MockStorageClient) CreateObject(ctx context.Context, bucket, name string, contents []byte, opts ...CreateOption) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "CreateObject",
		Params: []any{bucket, name, string(contents)},
	})

	if m.UploadErr != nil {
		return m.UploadErr
	}
	return nil
}

func (m *MockStorageClient) GetObject(ctx context.Context, bucket, name string) (io.ReadCloser, map[string]string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "GetObject",
		Params: []any{bucket, name},
	})

	metadata := make(map[string]string, 0)
	if m.Metadata != nil {
		metadata = m.Metadata
	}

	if m.DownloadErr != nil {
		return nil, nil, m.DownloadErr
	}

	if m.MetadataErr != nil {
		return nil, nil, m.MetadataErr
	}

	return &BufferReadCloser{bytes.NewBufferString(m.DownloadData)}, metadata, nil
}

func (m *MockStorageClient) DeleteObject(ctx context.Context, bucket, name string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "DeleteObject",
		Params: []any{bucket, name},
	})

	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	return nil
}

func (m *MockStorageClient) ObjectsWithName(ctx context.Context, bucket, filename string) ([]string, error) {
	if m.ListObjectErr != nil {
		return nil, m.ListObjectErr
	}
	return m.ListObjectURIs, nil
}
