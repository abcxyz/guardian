package codereview

import "context"

// CodeReview defines the minimum interface for a code review platform.
type CodeReview interface {
	// AssignReviewers adds a set of principals to the review the proposed code
	// changes.
	AssignReviewers(ctx context.Context, input *AssignReviewersInput) error
}

// AssignReviewersInput defines the possible principals that can be assigned to
// the review.
type AssignReviewersInput struct {
	Teams []string
	Users []string
}
