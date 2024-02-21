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

package flags

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/testutil"
)

func TestAfterParse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      []string
		flags     *GitHubFlags
		wantFlags *GitHubFlags
		wantErr   string
	}{
		{
			name: "both_token_and_app_id_set_results_in_error",
			args: []string{
				"--github-token=1234",
				"--github-app-id=987",
			},
			flags: &GitHubFlags{},
			wantFlags: &GitHubFlags{
				FlagGitHubToken: "1234",
				FlagGitHubAppID: "987",
			},
			wantErr: "only one of github token or github app id are allowed",
		},
		{
			name:      "neither_token_nor_app_id_set_results_in_error",
			args:      []string{},
			flags:     &GitHubFlags{},
			wantFlags: &GitHubFlags{},
			wantErr:   "one of github token or github app id are required",
		},
		{
			name: "just_token_parses_without_error",
			args: []string{
				"--github-token=1234",
				"--github-owner=my-org",
				"--github-repo=my-repo",
			},
			flags: &GitHubFlags{},
			wantFlags: &GitHubFlags{
				FlagGitHubToken: "1234",
				FlagGitHubOwner: "my-org",
				FlagGitHubRepo:  "my-repo",
			},
			wantErr: "",
		},
		{
			name: "just_github_app_parses_without_error",
			args: []string{
				"--github-app-id=1234",
				"--github-app-installation-id=7654",
				"--github-app-private-key-resource-name=projects/foo/secrets/pk/versions/1",
				"--github-owner=my-org",
				"--github-repo=my-repo",
			},
			flags: &GitHubFlags{},
			wantFlags: &GitHubFlags{
				FlagGitHubAppID:                     "1234",
				FlagGitHubAppInstallationID:         "7654",
				FlagGitHubAppPrivateKeyResourceName: "projects/foo/secrets/pk/versions/1",
				FlagGitHubOwner:                     "my-org",
				FlagGitHubRepo:                      "my-repo",
			},
			wantErr: "",
		},
		{
			name: "github_app_id_without_installation_id_results_in_error",
			args: []string{
				"--github-app-id=1234",
				"--github-app-private-key-resource-name=projects/foo/secrets/pk/versions/1",
				"--github-owner=my-org",
				"--github-repo=my-repo",
			},
			flags: &GitHubFlags{},
			wantFlags: &GitHubFlags{
				FlagGitHubAppID:                     "1234",
				FlagGitHubAppPrivateKeyResourceName: "projects/foo/secrets/pk/versions/1",
				FlagGitHubOwner:                     "my-org",
				FlagGitHubRepo:                      "my-repo",
			},
			wantErr: "a github app installation id is required when using a github app id",
		},
		{
			name: "github_app_id_without_private_key_results_in_error",
			args: []string{
				"--github-app-id=1234",
				"--github-app-installation-id=7654",
				"--github-owner=my-org",
				"--github-repo=my-repo",
			},
			flags: &GitHubFlags{},
			wantFlags: &GitHubFlags{
				FlagGitHubAppID:             "1234",
				FlagGitHubAppInstallationID: "7654",
				FlagGitHubOwner:             "my-org",
				FlagGitHubRepo:              "my-repo",
			},
			wantErr: "a github app private key resource name is required when using a github app id",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			set := cli.NewFlagSet()
			tc.flags.Register(set)
			err := set.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected error (-got, +want):\n%s", diff)
			}
			if diff := cmp.Diff(tc.flags, tc.wantFlags); diff != "" {
				t.Errorf("unexpected result (-got, +want):\n%s", diff)
			}
		})
	}
}
