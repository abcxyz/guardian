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

package platform

import (
	"context"
	"fmt"
	"sync"
)

type Request struct {
	Name   string
	Params []any
}

type MockPlatform struct {
	reqMu sync.Mutex
	Reqs  []*Request

	AssignReviewersErr error
	UserReviewers      []string
	TeamReviewers      []string
}

func (m *MockPlatform) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	m.Reqs = append(m.Reqs, &Request{
		Name:   "AssignReviewers",
		Params: []any{inputs.Users, inputs.Teams},
	})

	if m.AssignReviewersErr != nil {
		return nil, m.AssignReviewersErr
	}

	return &AssignReviewersResult{
		Users: m.UserReviewers,
		Teams: m.TeamReviewers,
	}, nil
}
