package policy

import (
	"context"
)

// CodeReviewPlatform is an abstraction of API's for code review platforms.
type CodeReviewPlatform interface {
	RequestReviewers(context context.Context, users, teams []string) error
}
