package platform

import (
	"context"
	"sync"
)

var _ Platform = (*MockPlatform)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockPlatform struct {
	reqMu sync.Mutex
	Reqs  []*Request

	AssignReviewersErr error
}

func (m *MockPlatform) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "AssignReviewers",
		Params: []any{inputs},
	})

	if m.AssignReviewersErr != nil {
		return nil, m.AssignReviewersErr
	}

	return &AssignReviewersResult{
		Teams: inputs.Teams,
		Users: inputs.Users,
	}, nil
}
