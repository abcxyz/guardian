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
		expErr      string
	}{
		{
			name:        "success",
			command:     "bash",
			args:        []string{"-c", "echo \"this is a test\""},
			expStdout:   "this is a test",
			expStderr:   "",
			expExitCode: 0,
			expErr:      "",
		},
		{
			name:        "returns_stderr",
			command:     "bash",
			args:        []string{"-c", "echo stdout && echo stderr >&2 && exit 1"},
			expStdout:   "stdout",
			expStderr:   "stderr",
			expExitCode: 1,
			expErr:      "failed to run command: exit status 1",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, exitCode, err := Run(ctx, tc.command, tc.args, "")
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Errorf("unexpected err: %s", diff)
			}
			if got, want := strings.TrimSpace(string(stdout)), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(string(stderr)), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := exitCode, tc.expExitCode; got != want {
				t.Errorf("expected %d to equal %d", got, want)
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
		expErr       string
	}{
		{
			name:      "cancels_context_after_2s",
			command:   "sleep",
			args:      []string{"5"},
			expStdout: "",
			expStderr: "",
			expErr:    "failed to run command: signal:",
		},
		{
			name:         "cancels_context_before",
			command:      "sleep",
			args:         []string{"5"},
			cancelBefore: true,
			expStdout:    "",
			expStderr:    "",
			expErr:       "failed to start command: context canceled",
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

			stdout, stderr, _, err := Run(ctx, tc.command, tc.args, "")
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Errorf("unexpected err: %s", diff)
			}
			if got, want := strings.TrimSpace(string(stdout)), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(string(stderr)), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}
