package platform

import (
	"context"
	"sync"
)

var _ Platform = (*MockGitHub)(nil)

type Request struct {
	Name   string
	Params []any
}

type MockGitHub struct {
	reqMu sync.Mutex
	Reqs  []*Request

	Owner  string
	Repo   string
	Number int

	AssignReviewersErr error
	UserReviewers      []string
	TeamReviewers      []string
}

func (m *MockGitHub) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	m.Reqs = append(m.Reqs, &Request{
		Name:   "AssignReviewers",
		Params: []any{m.Owner, m.Repo, m.Number, inputs.Users, inputs.Teams},
	})

	if m.AssignReviewersErr != nil {
		return nil, m.AssignReviewersErr
	}

	return &AssignReviewersResult{
		Users: m.UserReviewers,
		Teams: m.TeamReviewers,
	}, nil
}
