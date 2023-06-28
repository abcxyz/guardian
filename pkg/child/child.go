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
	"strings"
	"time"
)

// Child is a child process.
type Child struct {
	command string
	args    []string
	cmd     *exec.Cmd

	workingDir string
}

// New creates a new child process.
func New(ctx context.Context, command string, args []string, opts ...Option) (*Child, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("failed to locate command exec path: %w", err)
	}

	c := &Child{
		command: command,
		args:    args,
	}

	cmd := exec.CommandContext(ctx, path)
	c.cmd = cmd
	c.setSysProcAttr()
	c.setCancel()

	for _, opt := range opts {
		if opt != nil {
			c = opt(c)
		}
	}

	if c.workingDir != "" {
		c.cmd.Dir = c.workingDir
	}

	return c, nil
}

// Run executes the child process with the provided arguments.
func (c *Child) Run(ctx context.Context) ([]byte, []byte, int, error) {
	select {
	case <-ctx.Done():
		return nil, nil, 1, fmt.Errorf("failed to run command: %w", ctx.Err())
	default:
	}

	c.cmd.Args = append(c.cmd.Args, c.args...)

	// add small wait delay to kill subprocesses if context is canceled
	// https://github.com/golang/go/issues/23019
	// https://github.com/golang/go/issues/50436
	c.cmd.WaitDelay = 2 * time.Second

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	c.cmd.Stdout = stdout
	c.cmd.Stderr = stderr

	if err := c.cmd.Start(); err != nil {
		return nil, nil, c.cmd.ProcessState.ExitCode(), fmt.Errorf("failed to start command: %w", err)
	}

	if err := c.cmd.Wait(); err != nil {
		return stdout.Bytes(), stderr.Bytes(), c.cmd.ProcessState.ExitCode(), fmt.Errorf("failed to run command: %w", err)
	}

	return stdout.Bytes(), stderr.Bytes(), c.cmd.ProcessState.ExitCode(), nil
}

// Command prints the command and args for the child process.
func (c *Child) Command() string {
	list := append([]string{c.command}, c.args...)
	return strings.Join(list, " ")
}
