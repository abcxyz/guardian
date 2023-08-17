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

package initialize

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestInitProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name                           string
		config                         *Config
		directory                      string
		flagIsGitHubActions            bool
		flagGitHubOwner                string
		flagGitHubRepo                 string
		flagPullRequestNumber          int
		flagDestRef                    string
		flagSourceRef                  string
		flagDeleteOutdatedPlanComments bool
		flagSkipDetectChanges          bool
		flagFormat                     string
		flagRequiredPermissions        []string
		gitClient                      *git.MockGitClient
		err                            string
		expGitHubClientReqs            []*github.Request
		expStdout                      string
		expStderr                      string
	}{
		{
			name:                  "success",
			directory:             "testdata",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 1,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expGitHubClientReqs: nil,
			expStdout:           "testdata/backends/project1\ntestdata/backends/project2",
		},
		{
			name:                           "deletes_outdated_plan_comments",
			directory:                      "testdata",
			flagDeleteOutdatedPlanComments: true,
			flagFormat:                     "text",
			flagIsGitHubActions:            true,
			flagGitHubOwner:                "owner",
			flagGitHubRepo:                 "repo",
			flagPullRequestNumber:          2,
			flagDestRef:                    "main",
			flagSourceRef:                  "ldap/feature",
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 2},
				},
			},
			expStdout: "testdata/backends/project1\ntestdata/backends/project2",
		},
		{
			name:                  "returns_json",
			directory:             "testdata",
			flagFormat:            "json",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expGitHubClientReqs: nil,
			expStdout:           "[\"testdata/backends/project1\",\"testdata/backends/project2\"]",
		},
		{
			name:                  "invalid_format",
			directory:             "testdata",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			flagFormat:            "yaml",
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			err: "invalid format flag: yaml",
		},
		{
			name:                  "skips_detect_changes",
			directory:             "testdata",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 1,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			flagSkipDetectChanges: true,
			gitClient:             &git.MockGitClient{},
			expGitHubClientReqs:   nil,
			expStdout:             "testdata/backends/project1\ntestdata/backends/project2",
		},
		{
			name:                    "requires_permissions",
			config:                  &Config{Actor: "testuser"},
			directory:               "testdata",
			flagIsGitHubActions:     true,
			flagGitHubOwner:         "owner",
			flagGitHubRepo:          "repo",
			flagPullRequestNumber:   1,
			flagDestRef:             "main",
			flagSourceRef:           "ldap/feature",
			flagRequiredPermissions: []string{"admin"},
			flagSkipDetectChanges:   true,
			gitClient:               &git.MockGitClient{},
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
		{
			name:                  "errors",
			directory:             "testdata",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 2,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			gitClient: &git.MockGitClient{
				DiffErr: fmt.Errorf("failed to run git diff"),
			},
			expGitHubClientReqs: nil,
			err:                 "failed to find git diff directories: failed to run git diff",
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

			c := &InitCommand{
				directory: tc.directory,
				cfg:       tc.config,

				flagPullRequestNumber:          tc.flagPullRequestNumber,
				flagDeleteOutdatedPlanComments: tc.flagDeleteOutdatedPlanComments,
				flagFormat:                     tc.flagFormat,
				flagDestRef:                    tc.flagDestRef,
				flagSourceRef:                  tc.flagSourceRef,
				flagSkipDetectChanges:          tc.flagSkipDetectChanges,
				flagRequiredPermissions:        tc.flagRequiredPermissions,
				GitHubFlags: flags.GitHubFlags{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					FlagGitHubOwner:     tc.flagGitHubOwner,
					FlagGitHubRepo:      tc.flagGitHubRepo,
				},
				actions:      actions,
				gitClient:    tc.gitClient,
				gitHubClient: gitHubClient,
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
