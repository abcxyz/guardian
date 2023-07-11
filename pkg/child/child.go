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
	"io"
	"os/exec"
	"time"
)

// RunConfig are the inputs for a run operation.
type RunConfig struct {
	WorkingDir string
	Command    string
	Args       []string
}

// RunResult is the response from a run operation.
type RunResult struct {
	Stdout   io.Reader
	Stderr   io.Reader
	ExitCode int
}

// Run executes a child process with the provided arguments.
func Run(ctx context.Context, cfg *RunConfig) (*RunResult, error) {
	path, err := exec.LookPath(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to locate command exec path: %w", err)
	}

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, path)
	setSysProcAttr(cmd)
	setCancel(cmd)

	if v := cfg.WorkingDir; v != "" {
		cmd.Dir = v
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Args = append(cmd.Args, cfg.Args...)

	// add small wait delay to kill subprocesses if context is canceled
	// https://github.com/golang/go/issues/23019
	// https://github.com/golang/go/issues/50436
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Start(); err != nil {
		return &RunResult{
			Stdout:   &stdout,
			Stderr:   &stderr,
			ExitCode: cmd.ProcessState.ExitCode(),
		}, fmt.Errorf("failed to start command: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return &RunResult{
			Stdout:   &stdout,
			Stderr:   &stderr,
			ExitCode: cmd.ProcessState.ExitCode(),
		}, fmt.Errorf("failed to run command: %w", err)
	}

	return &RunResult{
		Stdout:   &stdout,
		Stderr:   &stderr,
		ExitCode: cmd.ProcessState.ExitCode(),
	}, nil
}
