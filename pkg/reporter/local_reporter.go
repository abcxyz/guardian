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
	"fmt"
	"io"
)

var _ Reporter = (*LocalReporter)(nil)

// mapping of operations to display text.
var stdoutOperationText = map[Operation]string{
	OperationPlan:    "PLAN",
	OperationApply:   "APPLY",
	OperationUnknown: "UNKNOWN",
}

// mapping of statuses to display text.
var stdoutStatusText = map[Status]string{
	StatusStart:     "STARTED",
	StatusSuccess:   "SUCCESS",
	StatusNoChanges: "NO CHANGES",
	StatusFailure:   "FAILED",
	StatusUnknown:   "UNKNOWN",
}

// LocalReporter implements the reporter interface for writing to stdout.
type LocalReporter struct {
	stdout io.Writer
}

// NewLocalReporter creates a new NewLocalReporter.
func NewLocalReporter(ctx context.Context, stdout io.Writer) (Reporter, error) {
	return &LocalReporter{
		stdout: stdout,
	}, nil
}

// CreateStatus write the status to stdout.
func (s *LocalReporter) CreateStatus(ctx context.Context, p *Params) error {
	operationText, ok := stdoutOperationText[p.Operation]
	if !ok {
		operationText = stdoutOperationText[OperationUnknown]
	}

	statusText, ok := stdoutStatusText[p.Status]
	if !ok {
		statusText = stdoutStatusText[StatusUnknown]
	}

	fmt.Fprintf(s.stdout, "%s - %s", operationText, statusText)
	return nil
}

// UpdateStatus write the status to stdout.
func (s *LocalReporter) UpdateStatus(ctx context.Context, p *Params) error {
	return s.CreateStatus(ctx, p)
}
