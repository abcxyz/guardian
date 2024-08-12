package policy

import (
	"context"
)

// CodeReviewPlatform is an abstraction of API's for code review platforms.
type CodeReviewPlatform interface {
	// RequestReviewers assigns a set of users and teams to review the proposed
	// code changes.
	RequestReviewers(context context.Context, users, teams []string) error
}
