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
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestRun(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			exitCode, err := Run(ctx, &RunConfig{
				Stdout:  &stdout,
				Stderr:  &stderr,
				Command: tc.command,
				Args:    tc.args,
			})
			if err != nil {
				if diff := testutil.DiffErrString(err, tc.err); diff != "" {
					t.Error(diff)
				}
				return
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := exitCode, tc.expExitCode; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		})
	}
}

func TestRun_Cancel(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

			if tc.cancelBefore {
				cancel()
			} else {
				defer cancel()
			}

			var stdout, stderr bytes.Buffer

			exitCode, err := Run(ctx, &RunConfig{
				Stdout:  &stdout,
				Stderr:  &stderr,
				Command: tc.command,
				Args:    tc.args,
			})
			if err != nil {
				if diff := testutil.DiffErrString(err, tc.err); diff != "" {
					t.Error(diff)
				}
				return
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
			if got, want := exitCode, tc.expExitCode; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		})
	}
}

func TestEnviron(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		osEnv       []string
		allowedKeys []string
		deniedKeys  []string
		overrideEnv []string

		exp []string
	}{
		{
			name: "nil_all",
			exp:  []string{},
		},
		{
			name:        "empty_all",
			osEnv:       []string{},
			allowedKeys: []string{},
			deniedKeys:  []string{},
			overrideEnv: []string{},
			exp:         []string{},
		},
		{
			name:        "empty_osenv",
			osEnv:       []string{},
			allowedKeys: []string{"allowed_key1", "allowed_key2"},
			deniedKeys:  []string{"denied_key2", "denied_key2"},
			overrideEnv: []string{},
			exp:         []string{},
		},
		{
			name:        "admits_allowed_keys",
			osEnv:       []string{"allowed_key1", "allowed_key2"},
			allowedKeys: []string{"allowed_key1", "allowed_key2"},
			deniedKeys:  []string{},
			overrideEnv: []string{},
			exp:         []string{"allowed_key1", "allowed_key2"},
		},
		{
			name:        "admits_allowed_keys_match",
			osEnv:       []string{"allowed_key1", "allowed_key2"},
			allowedKeys: []string{"allowed_*"},
			deniedKeys:  []string{},
			overrideEnv: []string{},
			exp:         []string{"allowed_key1", "allowed_key2"},
		},
		{
			name:        "admits_osenv_empty",
			osEnv:       []string{"allowed_key1", "allowed_key2"},
			allowedKeys: []string{},
			deniedKeys:  []string{},
			overrideEnv: []string{},
			exp:         []string{"allowed_key1", "allowed_key2"},
		},
		{
			name:        "rejects_denied_keys",
			osEnv:       []string{"denied_key1", "denied_key2"},
			allowedKeys: []string{},
			deniedKeys:  []string{"denied_key1", "denied_key2"},
			overrideEnv: []string{},
			exp:         []string{},
		},
		{
			name:        "rejects_denied_keys_match",
			osEnv:       []string{"denied_key1", "denied_key2"},
			allowedKeys: []string{},
			deniedKeys:  []string{"denied_*"},
			overrideEnv: []string{},
			exp:         []string{},
		},
		{
			name:        "deny_takes_precedence_over_allow",
			osEnv:       []string{"denied_key1", "denied_key2"},
			allowedKeys: []string{"denied_*"},
			deniedKeys:  []string{"denied_*"},
			overrideEnv: []string{},
			exp:         []string{},
		},
		{
			name: "allows_and_denies_corpus",
			osEnv: []string{
				"allowed_key1", "allowed_key2",
				"denied_key1", "denied_key2",
			},
			allowedKeys: []string{"allowed_*"},
			deniedKeys:  []string{"denied_*"},
			overrideEnv: []string{},
			exp:         []string{"allowed_key1", "allowed_key2"},
		},
		{
			name: "ignores_values",
			osEnv: []string{
				"allowed_key1=denied_value1", "allowed_key2=denied_value2",
				"denied_key1", "denied_key2",
			},
			allowedKeys: []string{"allowed_*"},
			deniedKeys:  []string{"denied_*"},
			overrideEnv: []string{},
			exp:         []string{"allowed_key1=denied_value1", "allowed_key2=denied_value2"},
		},
		{
			name:        "overrides_always",
			osEnv:       []string{},
			allowedKeys: []string{},
			deniedKeys:  []string{"*"},
			overrideEnv: []string{"override=value"},
			exp:         []string{"override=value"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Grab copies to make sure we don't modify these slices!
			originalOsEnv := append([]string{}, tc.osEnv...)
			originalAllowedKeys := append([]string{}, tc.allowedKeys...)
			originalDeniedKeys := append([]string{}, tc.deniedKeys...)
			originalOverrideEnv := append([]string{}, tc.overrideEnv...)

			got := environ(tc.osEnv, tc.allowedKeys, tc.deniedKeys, tc.overrideEnv)
			if diff := cmp.Diff(tc.exp, got); diff != "" {
				t.Errorf("unexpected environment (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.osEnv, originalOsEnv); tc.osEnv != nil && diff != "" {
				t.Errorf("osEnv was modified (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.allowedKeys, originalAllowedKeys); tc.allowedKeys != nil && diff != "" {
				t.Errorf("allowedKeys was modified (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.deniedKeys, originalDeniedKeys); tc.deniedKeys != nil && diff != "" {
				t.Errorf("deniedKeys was modified (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.overrideEnv, originalOverrideEnv); tc.overrideEnv != nil && diff != "" {
				t.Errorf("overrideEnv was modified (-want, +got):\n%s", diff)
			}
		})
	}
}
