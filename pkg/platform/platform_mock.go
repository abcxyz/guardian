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
