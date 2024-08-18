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

var _ PlanStorage = (*MockPlanStorageClient)(nil)

type PlanStorageRequest struct {
	Name   string
	Params []any
}

type MockPlanStorageClient struct {
	reqMu sync.Mutex
	Reqs  []*Request

	SavePlanErr   error
	GetPlanData   string
	GetPlanErr    error
	Metadata      map[string]string
	DeletePlanErr error
}

func (m *MockPlanStorageClient) SavePlan(ctx context.Context, name string, contents []byte, metadata map[string]string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "SavePlan",
		Params: []any{name, string(contents), metadata},
	})

	if m.SavePlanErr != nil {
		return m.SavePlanErr
	}
	return nil
}

func (m *MockPlanStorageClient) GetPlan(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "GetPlan",
		Params: []any{name},
	})

	metadata := make(map[string]string, 0)
	if m.Metadata != nil {
		metadata = m.Metadata
	}

	if m.GetPlanErr != nil {
		return nil, nil, m.GetPlanErr
	}

	return &BufferReadCloser{bytes.NewBufferString(m.GetPlanData)}, metadata, nil
}

func (m *MockPlanStorageClient) DeletePlan(ctx context.Context, name string) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "DeletePlan",
		Params: []any{name},
	})

	if m.DeletePlanErr != nil {
		return m.DeletePlanErr
	}
	return nil
}
