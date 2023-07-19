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

package plan

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestPlanInitProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name                     string
		directory                string
		flagGitHubToken          string
		flagIsGitHubActions      bool
		flagGitHubOwner          string
		flagGitHubRepo           string
		flagPullRequestNumber    int
		flagDestRef              string
		flagSourceRef            string
		flagKeepOutdatedComments bool
		flagFormat               string
		flagRetryMaxAttempts     uint64
		flagRetryInitialDelay    time.Duration
		flagRetryMaxDelay        time.Duration
		gitClient                *git.MockGitClient
		err                      string
		expGitHubClientReqs      []*github.Request
		expStdout                string
		expStderr                string
	}{
		{
			name:                     "success",
			directory:                "testdata",
			flagGitHubToken:          "github-token",
			flagKeepOutdatedComments: false,
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    1,
			flagDestRef:              "main",
			flagSourceRef:            "ldap/feature",
			flagRetryMaxAttempts:     3,
			flagRetryInitialDelay:    2 * time.Second,
			flagRetryMaxDelay:        10 * time.Second,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 1},
				},
			},
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expStdout: "testdata/backends/project1\ntestdata/backends/project2",
		},
		{
			name:                     "skips_deleting_comments",
			directory:                "testdata",
			flagGitHubToken:          "github-token",
			flagKeepOutdatedComments: true,
			flagFormat:               "text",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    2,
			flagDestRef:              "main",
			flagSourceRef:            "ldap/feature",
			flagRetryMaxAttempts:     3,
			flagRetryInitialDelay:    2 * time.Second,
			flagRetryMaxDelay:        10 * time.Second,
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
			name:                  "returns_json",
			directory:             "testdata",
			flagGitHubToken:       "github-token",
			flagFormat:            "json",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			flagRetryMaxAttempts:  3,
			flagRetryInitialDelay: 2 * time.Second,
			flagRetryMaxDelay:     10 * time.Second,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 3},
				},
			},
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			expStdout: "[\"testdata/backends/project1\",\"testdata/backends/project2\"]",
		},
		{
			name:                  "invalid_format",
			directory:             "testdata",
			flagGitHubToken:       "github-token",
			flagFormat:            "yaml",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			flagRetryMaxAttempts:  3,
			flagRetryInitialDelay: 2 * time.Second,
			flagRetryMaxDelay:     10 * time.Second,
			gitClient: &git.MockGitClient{
				DiffResp: []string{
					path.Join(cwd, "testdata/backends/project1"),
					path.Join(cwd, "testdata/backends/project2"),
				},
			},
			err: "invalid format flag: yaml",
		},
		{
			name:                  "errors",
			directory:             "testdata",
			flagGitHubToken:       "github-token",
			flagIsGitHubActions:   true,
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 2,
			flagDestRef:           "main",
			flagSourceRef:         "ldap/feature",
			flagRetryMaxAttempts:  3,
			flagRetryInitialDelay: 2 * time.Second,
			flagRetryMaxDelay:     10 * time.Second,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 2},
				},
			},
			gitClient: &git.MockGitClient{
				DiffErr: fmt.Errorf("failed to run git diff"),
			},
			err: "failed to find git diff directories: failed to run git diff",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			githubClient := &github.MockGitHubClient{}

			c := &PlanInitCommand{
				directory: tc.directory,

				flagPullRequestNumber:    tc.flagPullRequestNumber,
				flagKeepOutdatedComments: tc.flagKeepOutdatedComments,
				flagFormat:               tc.flagFormat,
				flagDestRef:              tc.flagDestRef,
				flagSourceRef:            tc.flagSourceRef,
				GitHubFlags: flags.GitHubFlags{
					FlagGitHubToken:     tc.flagGitHubToken,
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					FlagGitHubOwner:     tc.flagGitHubOwner,
					FlagGitHubRepo:      tc.flagGitHubRepo,
				},
				RetryFlags: flags.RetryFlags{
					FlagRetryMaxAttempts:  tc.flagRetryMaxAttempts,
					FlagRetryInitialDelay: tc.flagRetryInitialDelay,
					FlagRetryMaxDelay:     tc.flagRetryMaxDelay,
				},

				gitClient:    tc.gitClient,
				githubClient: githubClient,
			}

			_, stdout, stderr := c.Pipe()

			err = c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(githubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
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
