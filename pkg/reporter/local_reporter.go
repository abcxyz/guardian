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
	"strings"
)

var _ Reporter = (*LocalReporter)(nil)

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

// CreateStatus writes the status to stdout.
func (s *LocalReporter) CreateStatus(ctx context.Context, st Status, p *Params) error {
	if p.Operation != "" {
		fmt.Fprintf(s.stdout, "%s", strings.ToUpper(p.Operation))
	}

	fmt.Fprintf(s.stdout, " - %s", st)

	if p.Message != "" {
		fmt.Fprintf(s.stdout, ": %s", p.Message)
	}

	fmt.Fprintf(s.stdout, "\n")

	return nil
}

// ClearStatus is a no-op because we dont want to clear stdout.
func (s *LocalReporter) ClearStatus(ctx context.Context) error {
	return nil
}
