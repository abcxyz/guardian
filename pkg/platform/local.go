package platform

import "context"

var _ Platform = (*Local)(nil)

// // Local implements the Platform interface for running Guardian locally.
type Local struct{}

// NewLocal creates a new Local instance.
func NewLocal(ctx context.Context) *Local {
	return &Local{}
}
