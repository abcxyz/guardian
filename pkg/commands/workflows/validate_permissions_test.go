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

package workflows

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestValidatePermissionsAfterParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "validate_github_flags",
			args: []string{},
			err:  "missing flag: github-owner is required\nmissing flag: github-repo is required",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &ValidatePermissionsCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestValidatePermissionsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                   string
		config                 *ValidatePermissionsConfig
		flagIsGitHubActions    bool
		flagGitHubOwner        string
		flagGitHubRepo         string
		flagAllowedPermissions []string
		err                    string
		expGitHubClientReqs    []*github.Request
		expStdout              string
		expStderr              string
	}{
		{
			name:                   "allowed",
			config:                 &ValidatePermissionsConfig{Actor: "testuser"},
			flagIsGitHubActions:    true,
			flagGitHubOwner:        "owner",
			flagGitHubRepo:         "repo",
			flagAllowedPermissions: []string{"read"},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RepoUserPermissionLevel",
					Params: []any{"owner", "repo", "testuser"},
				},
			},
			err:       "",
			expStdout: "",
			expStderr: "",
		},
		{
			name:                   "denied",
			config:                 &ValidatePermissionsConfig{Actor: "testuser"},
			flagIsGitHubActions:    true,
			flagGitHubOwner:        "owner",
			flagGitHubRepo:         "repo",
			flagAllowedPermissions: []string{"admin"},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "RepoUserPermissionLevel",
					Params: []any{"owner", "repo", "testuser"},
				},
			},
			err:       "testuser does not have the required permissions to run this command.\n\nRequired permissions are [\"admin\"]",
			expStdout: "",
			expStderr: "",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actions := githubactions.New(githubactions.WithWriter(os.Stdout))
			gitHubClient := &github.MockGitHubClient{
				RepoPermissionLevel: "read",
			}

			c := &ValidatePermissionsCommand{
				cfg: tc.config,

				GitHubFlags: flags.GitHubFlags{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					FlagGitHubOwner:     tc.flagGitHubOwner,
					FlagGitHubRepo:      tc.flagGitHubRepo,
				},
				flagAllowedPermissions: tc.flagAllowedPermissions,
				actions:                actions,
				gitHubClient:           gitHubClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected stdout\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected stderr\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}
