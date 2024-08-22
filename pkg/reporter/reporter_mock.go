// Copyright 2024 The Authors (see AUTHORS file)
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

package reporter

import (
	"context"
	"sync"
)

var _ Reporter = (*MockReporter)(nil)

type Request struct {
	Name   string
	Params []any
}

// MockReporter implements the reporter interface for mocking in unit tests.
type MockReporter struct {
	reqMu sync.Mutex
	Reqs  []*Request

	StatusErr             error
	EntrypointsSummaryErr error
	ClearErr              error
}

// Status implements the Status function.
func (m *MockReporter) Status(ctx context.Context, s Status, p *StatusParams) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	// make a copy to prevent outside modification
	pCopy := new(StatusParams)
	*pCopy = *p

	m.Reqs = append(m.Reqs, &Request{
		Name:   "Status",
		Params: []any{s, pCopy},
	})

	return m.StatusErr
}

// EntrypointsSummary implements the EntrypointsSummary function.
func (m *MockReporter) EntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	// make a copy to prevent outside modification
	pCopy := new(EntrypointsSummaryParams)
	*pCopy = *p

	m.Reqs = append(m.Reqs, &Request{
		Name:   "EntrypointsSummary",
		Params: []any{pCopy},
	})

	return m.EntrypointsSummaryErr
}

// Cleartatus implements the Clear function.
func (m *MockReporter) Clear(ctx context.Context) error {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name: "Clear",
	})

	return m.ClearErr
}
