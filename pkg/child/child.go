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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/abcxyz/pkg/logging"
)

// RunConfig are the inputs for a run operation.
type RunConfig struct {
	Stdout     io.Writer
	Stderr     io.Writer
	WorkingDir string
	Command    string
	Args       []string
}

// Run executes a child process with the provided arguments.
func Run(ctx context.Context, cfg *RunConfig) (int, error) {
	logger := logging.FromContext(ctx)

	path, err := exec.LookPath(cfg.Command)
	if err != nil {
		return -1, fmt.Errorf("failed to locate command exec path: %w", err)
	}

	cmd := exec.CommandContext(ctx, path)
	setSysProcAttr(cmd)
	setCancel(cmd)

	if v := cfg.WorkingDir; v != "" {
		cmd.Dir = v
	}

	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}

	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	cmd.Args = append(cmd.Args, cfg.Args...)

	// add small wait delay to kill subprocesses if context is canceled
	// https://github.com/golang/go/issues/23019
	// https://github.com/golang/go/issues/50436
	cmd.WaitDelay = 2 * time.Second

	logger.Debugf("running command %s %s", cfg.Command, strings.Join(cfg.Args, " "))

	if err := cmd.Start(); err != nil {
		return cmd.ProcessState.ExitCode(), fmt.Errorf("failed to start command: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return cmd.ProcessState.ExitCode(), fmt.Errorf("failed to run command: %w", err)
	}

	return cmd.ProcessState.ExitCode(), nil
}
