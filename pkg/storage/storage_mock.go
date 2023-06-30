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
	"context"
	"fmt"
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

	UploadErr    string
	GetData      string
	GetErr       string
	GetLimitData string
	GetLimitErr  string
	Metadata     map[string]string
	MetadataErr  string
	DeleteErr    string
}

func (m *MockStorageClient) UploadObject(ctx context.Context, bucket, name string, contents []byte, contentType string, metadata map[string]string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "UploadObject",
		Params: []any{bucket, name, contents, contentType, metadata},
	})

	if m.UploadErr != "" {
		return fmt.Errorf("%s", m.UploadErr)
	}
	return nil
}

func (m *MockStorageClient) GetObject(ctx context.Context, bucket, name string) ([]byte, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "GetObject",
		Params: []any{bucket, name},
	})

	if m.GetErr != "" {
		return nil, fmt.Errorf("%s", m.GetErr)
	}
	return []byte(m.GetData), nil
}

func (m *MockStorageClient) GetObjectWithLimit(ctx context.Context, bucket, name string, limit int64) ([]byte, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "GetObjectWithLimit",
		Params: []any{bucket, name},
	})

	if m.GetLimitErr != "" {
		return nil, fmt.Errorf("%s", m.GetLimitErr)
	}
	return []byte(m.GetLimitData), nil
}

func (m *MockStorageClient) GetMetadata(ctx context.Context, bucket, name string) (map[string]string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "GetMetadata",
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

func (m *MockStorageClient) DeleteObject(ctx context.Context, bucket, name string, ignoreNotFound bool) error {
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
