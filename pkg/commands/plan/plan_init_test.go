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
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestPlanInitProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                       string
		flagGitHubToken            string
		flagGitHubAction           bool
		flagGitHubOwner            string
		flagGitHubRepo             string
		flagPullRequestNumber      int
		flagDestRef                string
		flagSourceRef              string
		flagDirectories            []string
		flagDeleteOutdatedComments bool
		flagJSON                   bool
		flagMaxRetries             uint64
		flagInitialRetryDelay      time.Duration
		flagMaxRetryDelay          time.Duration
		entrypoints                []string
		config                     *Config
		gitClient                  *git.MockGitClient
		githubClient               *github.MockGitHubClient
		err                        string
		expGitHubClientReqs        []*github.Request
		expStdout                  string
		expStderr                  string
	}{
		{
			name:                       "success",
			flagGitHubToken:            "github-token",
			flagDeleteOutdatedComments: true,
			flagJSON:                   false,
			flagGitHubAction:           true,
			flagGitHubOwner:            "owner",
			flagGitHubRepo:             "repo",
			flagPullRequestNumber:      1,
			flagDestRef:                "main",
			flagSourceRef:              "ldap/feature",
			entrypoints:                []string{"dir1", "dir2"},
			flagMaxRetries:             3,
			flagInitialRetryDelay:      2 * time.Second,
			flagMaxRetryDelay:          10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: int64(1),
			},
			gitClient:    &git.MockGitClient{DiffResp: []string{"dir1", "dir2"}},
			githubClient: &github.MockGitHubClient{},
			expStdout:    "dir1\ndir2",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", 1},
				},
			},
		},
		{
			name:                       "skips_deleting_comments",
			flagGitHubToken:            "github-token",
			flagDeleteOutdatedComments: false,
			flagJSON:                   false,
			flagGitHubAction:           true,
			flagGitHubOwner:            "owner",
			flagGitHubRepo:             "repo",
			flagPullRequestNumber:      2,
			flagDestRef:                "main",
			flagSourceRef:              "ldap/feature",
			entrypoints:                []string{"dir3", "dir4"},
			flagMaxRetries:             3,
			flagInitialRetryDelay:      2 * time.Second,
			flagMaxRetryDelay:          10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: int64(1),
			},
			gitClient:           &git.MockGitClient{DiffResp: []string{"dir3", "dir4"}},
			githubClient:        &github.MockGitHubClient{},
			expGitHubClientReqs: nil,
			expStdout:           "dir3\ndir4",
		},
		{
			name:                       "returns_json",
			flagGitHubToken:            "github-token",
			flagDeleteOutdatedComments: false,
			flagJSON:                   true,
			flagGitHubAction:           true,
			flagGitHubOwner:            "owner",
			flagGitHubRepo:             "repo",
			flagPullRequestNumber:      3,
			flagDestRef:                "main",
			flagSourceRef:              "ldap/feature",
			entrypoints:                []string{"dir5", "dir6"},
			flagMaxRetries:             3,
			flagInitialRetryDelay:      2 * time.Second,
			flagMaxRetryDelay:          10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: int64(1),
			},
			gitClient:    &git.MockGitClient{DiffResp: []string{"dir5", "dir6"}},
			githubClient: &github.MockGitHubClient{},
			expStdout:    "[\"dir5\",\"dir6\"]",
		},
		{
			name:                       "errors",
			flagGitHubToken:            "github-token",
			flagDeleteOutdatedComments: false,
			flagGitHubAction:           true,
			flagGitHubOwner:            "owner",
			flagGitHubRepo:             "repo",
			flagPullRequestNumber:      2,
			flagDestRef:                "main",
			flagSourceRef:              "ldap/feature",
			entrypoints:                []string{"dir3", "dir4"},
			flagMaxRetries:             3,
			flagInitialRetryDelay:      2 * time.Second,
			flagMaxRetryDelay:          10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: int64(1),
			},
			gitClient:    &git.MockGitClient{DiffErr: fmt.Errorf("failed to run git diff")},
			githubClient: &github.MockGitHubClient{},
			err:          "failed to find git diff directories: failed to run git diff",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actions := githubactions.New(githubactions.WithWriter(os.Stdout))

			c := &PlanInitCommand{
				cfg: tc.config,

				workingDir:  "",
				entrypoints: tc.entrypoints,

				flagPullRequestNumber:      tc.flagPullRequestNumber,
				flagDirectories:            tc.flagDirectories,
				flagDeleteOutdatedComments: tc.flagDeleteOutdatedComments,
				flagJSON:                   tc.flagJSON,
				flagDestRef:                tc.flagDestRef,
				flagSourceRef:              tc.flagSourceRef,
				GitHubFlags: flags.GitHubFlags{
					FlagGitHubToken:  tc.flagGitHubToken,
					FlagGitHubAction: tc.flagGitHubAction,
					FlagGitHubOwner:  tc.flagGitHubOwner,
					FlagGitHubRepo:   tc.flagGitHubRepo,
				},
				RetryFlags: flags.RetryFlags{
					FlagMaxRetries:        tc.flagMaxRetries,
					FlagInitialRetryDelay: tc.flagInitialRetryDelay,
					FlagMaxRetryDelay:     tc.flagMaxRetryDelay,
				},

				actions:      actions,
				gitClient:    tc.gitClient,
				githubClient: tc.githubClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(tc.githubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
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
