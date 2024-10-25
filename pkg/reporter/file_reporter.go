package reporter

import (
	"context"
	"fmt"
	"os"

	"github.com/abcxyz/pkg/logging"
)

var _ Reporter = (*FileReporter)(nil)

const (
	statusFilename      = "status.md"
	ownerReadWritePerms = 0o600
)

type FileReporter struct{}

func NewFileReporter() (*FileReporter, error) {
	return &FileReporter{}, nil
}

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

func (f *FileReporter) EntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error {
	return nil
}

func (f *FileReporter) Clear(ctx context.Context) error {
	return nil
}
