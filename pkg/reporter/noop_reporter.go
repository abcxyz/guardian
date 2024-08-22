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

package reporter

import (
	"context"
)

var _ Reporter = (*NoopReporter)(nil)

// NoopReporter implements the reporter interface for no-op reporting.
type NoopReporter struct{}

// NewNoopReporter creates a new NewNoopReporter.
func NewNoopReporter(ctx context.Context) (Reporter, error) {
	return &NoopReporter{}, nil
}

// Status is a no-op.
func (s *NoopReporter) Status(ctx context.Context, st Status, p *StatusParams) error {
	return nil
}

// EntrypointsSummary is a no-op.
func (s *NoopReporter) EntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	return nil
}

// Clear is a no-op.
func (s *NoopReporter) Clear(ctx context.Context) error {
	return nil
}
