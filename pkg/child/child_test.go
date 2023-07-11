// Copyright 2023 The Authors (see AUTHORS file)
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

package child

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestChild_Run(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name        string
		command     string
		args        []string
		expStdout   string
		expStderr   string
		expExitCode int
		err         string
	}{
		{
			name:        "success",
			command:     "bash",
			args:        []string{"-c", "echo \"this is a test\""},
			expStdout:   "this is a test\n",
			expStderr:   "",
			expExitCode: 0,
		},
		{
			name:        "returns_stderr",
			command:     "bash",
			args:        []string{"-c", "echo stdout && echo stderr >&2 && exit 1"},
			expStdout:   "stdout\n",
			expStderr:   "stderr\n",
			expExitCode: 1,
			err:         "failed to run command: exit status 1",
		},
		{
			name:    "unknown_command",
			command: "thisisnotacommand",
			err:     "failed to locate command exec path: exec: \"thisisnotacommand\"",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Run(ctx, &RunConfig{WorkingDir: "", Command: tc.command, Args: tc.args})
			if err != nil {
				if diff := testutil.DiffErrString(err, tc.err); diff != "" {
					t.Errorf(diff)
				}
				return
			}

			stdout, err := io.ReadAll(result.Stdout)
			if err != nil {
				t.Fatalf("unexpected err: %s", err)
			}

			stderr, err := io.ReadAll(result.Stdout)
			if err != nil {
				t.Fatalf("unexpected err: %s", err)
			}

			if got, want := strings.TrimSpace(string(stdout)), strings.TrimSpace(tc.expStdout); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := strings.TrimSpace(string(stderr)), strings.TrimSpace(tc.expStderr); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := result.ExitCode, tc.expExitCode; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		})
	}
}

func TestChild_Run_Cancel(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name         string
		command      string
		args         []string
		cancelBefore bool
		expStdout    string
		expStderr    string
		expExitCode  int
		err          string
	}{
		{
			name:        "cancels_context_after_2s",
			command:     "sleep",
			args:        []string{"5"},
			expStdout:   "",
			expStderr:   "",
			expExitCode: -1,
			err:         "failed to run command: signal:",
		},
		{
			name:         "cancels_context_before",
			command:      "sleep",
			args:         []string{"5"},
			cancelBefore: true,
			expStdout:    "",
			expStderr:    "",
			expExitCode:  -1,
			err:          "failed to start command: context canceled",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

			if tc.cancelBefore {
				cancel()
			} else {
				defer cancel()
			}

			result, err := Run(ctx, &RunConfig{WorkingDir: "", Command: tc.command, Args: tc.args})
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			stdout, err := io.ReadAll(result.Stdout)
			if err != nil {
				t.Fatalf("unexpected err: %s", err)
			}

			stderr, err := io.ReadAll(result.Stdout)
			if err != nil {
				t.Fatalf("unexpected err: %s", err)
			}

			if got, want := strings.TrimSpace(string(stdout)), strings.TrimSpace(tc.expStdout); got != want {
				t.Errorf("stdout expected %s to be %s", got, want)
			}
			if got, want := strings.TrimSpace(string(stderr)), strings.TrimSpace(tc.expStderr); got != want {
				t.Errorf("stderr expected %s to be %s", got, want)
			}
			if got, want := result.ExitCode, tc.expExitCode; got != want {
				t.Errorf("exit code expected %d to be %d", got, want)
			}
		})
	}
}
