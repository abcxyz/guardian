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
	"os"

	"github.com/abcxyz/pkg/logging"
)

var _ Reporter = (*FileReporter)(nil)

const (
	statusFilename             = "status_comment.md"
	entrypointsSummaryFilename = "entrypoints_summary.md"
	ownerReadWritePerms        = 0o600
)

// FileReporter implements the reporter interface for writing comments to files.
type FileReporter struct{}

// NewFileReporter creates a new FileReporter.
func NewFileReporter() (*FileReporter, error) {
	return &FileReporter{}, nil
}

// Status implements the reporter Status function by writing the comment to a file.
func (f *FileReporter) Status(ctx context.Context, st Status, p *StatusParams) error {
	logger := logging.FromContext(ctx)

	msg, err := statusMessage(st, p, "", -1)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	if err := os.WriteFile(statusFilename, []byte(msg.String()), ownerReadWritePerms); err != nil {
		return fmt.Errorf("failed to write file")
	}

	logger.DebugContext(ctx, "wrote status comment to file", "statusFilename", statusFilename)
	return nil
}

// EntrypointsSummary implements the reporter EntrypointsSummary function by writing the comment to a file.
func (f *FileReporter) EntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error {
	logger := logging.FromContext(ctx)

	msg, err := entrypointsSummaryMessage(params, "")
	if err != nil {
		return fmt.Errorf("failed to generate entrypoints summary message: %w", err)
	}

	if err := os.WriteFile(entrypointsSummaryFilename, []byte(msg.String()), ownerReadWritePerms); err != nil {
		return fmt.Errorf("failed to write file")
	}

	logger.DebugContext(ctx, "wrote entrypoints summary to file", "entrypointsSummaryFilename", entrypointsSummaryFilename)
	return nil
}

// Clear is a no-op because workflow runners will cleanup the files at the end
// of execution.
func (f *FileReporter) Clear(ctx context.Context) error {
	return nil
}
