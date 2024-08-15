package codereview

import (
	"context"
)

var _ CodeReview = (*Local)(nil)

// Local implements the CodeReview interface for running Guardian locally.
type Local struct{}

// NewLocal creates a new Local instance.
func NewLocal(ctx context.Context) *Local {
	return &Local{}
}

// AssignReviewers is a no-op.
func (l *Local) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) error {
	return nil
}
