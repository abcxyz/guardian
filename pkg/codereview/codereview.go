package codereview

import "context"

type CodeReview interface {
	AssignReviewers(ctx context.Context, users, teams []string) error
}
