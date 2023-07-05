// Copyright 2023 The Authors (see AUTHORS file)
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

// Package child provides the functionality to execute child command line processes.
package child

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Run executes a child process with the provided arguments.
func Run(ctx context.Context, workingDir, command string, args []string) ([]byte, []byte, int, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, nil, 1, fmt.Errorf("failed to locate command exec path: %w", err)
	}

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, path)
	setSysProcAttr(cmd)
	setCancel(cmd)

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Args = append(cmd.Args, args...)

	// add small wait delay to kill subprocesses if context is canceled
	// https://github.com/golang/go/issues/23019
	// https://github.com/golang/go/issues/50436
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Start(); err != nil {
		return nil, nil, cmd.ProcessState.ExitCode(), fmt.Errorf("failed to start command: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return stdout.Bytes(), stderr.Bytes(), cmd.ProcessState.ExitCode(), fmt.Errorf("failed to run command: %w", err)
	}

	return stdout.Bytes(), stderr.Bytes(), cmd.ProcessState.ExitCode(), nil
}
