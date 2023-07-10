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
	"testing"
	"time"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestChild_Run(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name    string
		command string
		args    []string
		exp     *RunResult
		err     string
	}{
		{
			name:    "success",
			command: "bash",
			args:    []string{"-c", "echo \"this is a test\""},
			exp: &RunResult{
				Stdout:   []byte("this is a test\n"),
				Stderr:   []byte(""),
				ExitCode: 0,
			},
		},
		{
			name:    "returns_stderr",
			command: "bash",
			args:    []string{"-c", "echo stdout && echo stderr >&2 && exit 1"},
			exp: &RunResult{
				Stdout:   []byte("stdout\n"),
				Stderr:   []byte("stderr\n"),
				ExitCode: 1,
			},
			err: "failed to run command: exit status 1",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Run(ctx, &RunConfig{WorkingDir: "", Command: tc.command, Args: tc.args})
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
			if diff := cmp.Diff(result, tc.exp); diff != "" {
				t.Errorf("result differed from expected, (-got,+want): %s", diff)
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
		exp          *RunResult
		err          string
	}{
		{
			name:    "cancels_context_after_2s",
			command: "sleep",
			args:    []string{"5"},
			exp: &RunResult{
				Stdout:   []byte{},
				Stderr:   []byte{},
				ExitCode: -1,
			},
			err: "failed to run command: signal:",
		},
		{
			name:         "cancels_context_before",
			command:      "sleep",
			args:         []string{"5"},
			cancelBefore: true,
			exp: &RunResult{
				Stdout:   nil,
				Stderr:   nil,
				ExitCode: -1,
			},
			err: "failed to start command: context canceled",
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
			if diff := cmp.Diff(result, tc.exp); diff != "" {
				t.Errorf("result differed from expected, (-got,+want): %s", diff)
			}
		})
	}
}
