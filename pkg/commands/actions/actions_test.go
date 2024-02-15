// Copyright 2024 The Authors (see AUTHORS file)
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

package actions

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/pkg/testutil"
)

func TestWithActionsOutGroup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		flagIsGitHubActions bool
		actionout           *strings.Builder
		stdout              *strings.Builder
		msg                 string
		testFunc            func() error
		wantActionOut       string
		wantStdOut          string
		wantErr             string
	}{
		{
			name:                "action_disabled_error_pass_through",
			flagIsGitHubActions: false,
			actionout:           &strings.Builder{},
			stdout:              &strings.Builder{},
			msg:                 "MyActionGroup",
			testFunc: func() error {
				return fmt.Errorf("testFunc error")
			},
			wantActionOut: "",
			wantStdOut:    "MyActionGroup\n",
			wantErr:       "testFunc error",
		},
		{
			name:                "action_disabled_error_pass_nil",
			flagIsGitHubActions: false,
			actionout:           &strings.Builder{},
			stdout:              &strings.Builder{},
			msg:                 "MyActionGroup",
			testFunc: func() error {
				return nil
			},
			wantActionOut: "",
			wantStdOut:    "MyActionGroup\n",
			wantErr:       "",
		},
		{
			name:                "action_enabled_error_pass_through",
			flagIsGitHubActions: true,
			actionout:           &strings.Builder{},
			stdout:              &strings.Builder{},
			msg:                 "MyActionGroup",
			testFunc: func() error {
				return fmt.Errorf("testFunc error")
			},
			wantActionOut: "::group::MyActionGroup\n::endgroup::\n",
			wantStdOut:    "",
			wantErr:       "testFunc error",
		},
		{
			name:                "action_enabled_error_pass_nil",
			flagIsGitHubActions: true,
			actionout:           &strings.Builder{},
			stdout:              &strings.Builder{},
			msg:                 "MyActionGroup",
			testFunc: func() error {
				return nil
			},
			wantActionOut: "::group::MyActionGroup\n::endgroup::\n",
			wantStdOut:    "",
			wantErr:       "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			action := githubactions.New(githubactions.WithWriter(tc.actionout))

			cmd := &GitHubActionCommand{
				GitHubFlags: flags.GitHubFlags{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
				},
				Action: action,
			}

			cmd.SetStdout(tc.stdout)

			gotErr := cmd.WithActionsOutGroup(tc.msg, tc.testFunc)
			if diff := testutil.DiffErrString(gotErr, tc.wantErr); diff != "" {
				t.Errorf("unexpected result (-got, +want):\n%s", diff)
			}
			gotStdOut := tc.stdout.String()
			if diff := cmp.Diff(gotStdOut, tc.wantStdOut); diff != "" {
				t.Errorf("unexpected result (-got, +want):\n%s", diff)
			}
			gotActionOut := tc.actionout.String()
			if diff := cmp.Diff(gotActionOut, tc.wantActionOut); diff != "" {
				t.Errorf("unexpected result (-got, +want):\n%s", diff)
			}
		})
	}
}
