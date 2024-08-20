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

type MockGitHub struct {
	reqMu sync.Mutex
	Reqs  []*Request

	Owner             string
	Repo              string
	PullRequestNumber int

	AssignReviewersErr error
	UserReviewers      []string
	TeamReviewers      []string
}

func (m *MockGitHub) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	if inputs == nil {
		return nil, fmt.Errorf("inputs cannot be nil")
	}
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	for _, u := range inputs.Users {
		m.Reqs = append(m.Reqs, &Request{
			Name:   "AssignReviewers",
			Params: []any{m.Owner, m.Repo, m.PullRequestNumber, []string{u}, []string(nil)},
		})
	}
	for _, t := range inputs.Teams {
		m.Reqs = append(m.Reqs, &Request{
			Name:   "AssignReviewers",
			Params: []any{m.Owner, m.Repo, m.PullRequestNumber, []string(nil), []string{t}},
		})
	}

	if m.AssignReviewersErr != nil {
		return nil, m.AssignReviewersErr
	}

	return &AssignReviewersResult{
		Users: m.UserReviewers,
		Teams: m.TeamReviewers,
	}, nil
}
