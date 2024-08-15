package codereview

import (
	"context"
)

var _ CodeReview = (*Local)(nil)

type Local struct{}

func NewLocal(ctx context.Context) *Local {
	return &Local{}
}

func (l *Local) AssignReviewers(ctx context.Context, users, teams []string) error {
	// Do nothing
	return nil
}
