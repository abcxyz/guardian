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
	"path/filepath"
	"strings"
	"time"

	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/pkg/logging"
)

// overrideEnvVars are the environment variables to inject into the child
// process, no matter what the user configured. These take precedence over all
// other configurables.
var overrideEnvVars = []string{
	"GOOGLE_TERRAFORM_USERAGENT_EXTENSION=" + version.UserAgent,
	"TF_APPEND_USER_AGENT=" + version.UserAgent,
}

// RunConfig are the inputs for a run operation.
type RunConfig struct {
	Stdout     io.Writer
	Stderr     io.Writer
	WorkingDir string
	Command    string
	Args       []string

	// AllowedEnvKeys and DeniedEnvKeys respectively define an allow/deny list of
	// patterns for environment variable keys. Keys are matched using
	// [filepath.Match].
	AllowedEnvKeys []string
	DeniedEnvKeys  []string
}

// Run executes a child process with the provided arguments.
func Run(ctx context.Context, cfg *RunConfig) (int, error) {
	logger := logging.FromContext(ctx).
		With("working_dir", cfg.WorkingDir).
		With("command", cfg.Command).
		With("args", cfg.Args)

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

	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Args = append(cmd.Args, cfg.Args...)

	// Compute and set a custom environment for the child process.
	env := environ(os.Environ(), cfg.AllowedEnvKeys, cfg.DeniedEnvKeys, overrideEnvVars)
	logger.DebugContext(ctx, "computed environment", "env", env)
	cmd.Env = env

	// add small wait delay to kill subprocesses if context is canceled
	// https://github.com/golang/go/issues/23019
	// https://github.com/golang/go/issues/50436
	cmd.WaitDelay = 2 * time.Second

	logger.DebugContext(ctx, "command started")

	if err := cmd.Start(); err != nil {
		return cmd.ProcessState.ExitCode(), fmt.Errorf("failed to start command: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return cmd.ProcessState.ExitCode(), fmt.Errorf("failed to run command: %w", err)
	}

	exitCode := cmd.ProcessState.ExitCode()

	logger.DebugContext(ctx, "command completed", "exit_code", exitCode)

	return exitCode, nil
}

// environ compiles the appropriate environment to pass to the child process.
// The overridden environment is always added, even if not explicitly
// allowed/denied.
func environ(osEnv, allowedKeys, deniedKeys, overrideEnv []string) []string {
	finalEnv := make([]string, 0, len(osEnv)+len(overrideEnv))

	// Select keys that match the allow filter (if given) but not the deny filter.
	for _, v := range osEnv {
		k := strings.SplitN(v, "=", 2)[0]
		if (len(allowedKeys) == 0 || anyGlobMatch(k, allowedKeys)) &&
			!anyGlobMatch(k, deniedKeys) {
			finalEnv = append(finalEnv, v)
		}
	}

	// Add overrides at the end, after any filtering
	finalEnv = append(finalEnv, overrideEnv...)

	return finalEnv
}

func anyGlobMatch(s string, patterns []string) bool {
	// Short-circuit path matching logic for match-all.
	for _, p := range patterns {
		if p == "*" || p == ".*" {
			return true
		}
	}

	// Now do the slower lookup.
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, s); ok {
			return true
		}
	}
	return false
}
