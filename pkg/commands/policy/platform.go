// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"

	"github.com/abcxyz/pkg/logging"
)

// CodeReviewPlatform is an abstraction of API's for code review platforms.
type CodeReviewPlatform interface {
	// RequestReviewers assigns a set of users and teams to review the proposed
	// code changes.
	RequestReviewers(ctx context.Context, users, teams []string) error
}

// Local implements the CodeReviewPlatform interface.
type Local struct{}

// RequestReviewers skips calls to external API's and has no-op.
func (l *Local) RequestReviewers(ctx context.Context, users, teams []string) error {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "skipping request reviewers")
	return nil
}
