// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package platform defines interfaces for interacting with code review
// platforms.
package platform

import "context"

// ChangeRequest defines the minimum interface for a code review platform's
// change request.
type ChangeRequest interface {
	// AssignReviewers adds a set of principals to review the proposed code
	// changes.
	AssignReviewers(ctx context.Context, input *AssignReviewersInput) error
}

// AssignReviewersInput defines the possible principals that can be assigned to
// the review.
type AssignReviewersInput struct {
	Teams []string
	Users []string
}
