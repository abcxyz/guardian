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
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestNormalizeWorkingDir(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dir  string
		exp  string
		err  string
	}{
		{
			name: "current_dir",
			dir:  ".",
			exp:  "",
		},
		{
			name: "relative_current_dir",
			dir:  "../plan",
			exp:  "",
		},
		{
			name: "child_dir",
			dir:  "./testdata/test",
			exp:  "testdata/test",
		},
		{
			name: "empty",
			dir:  "",
			exp:  "",
		},
		{
			name: "non_child_dir",
			dir:  "../another",
			err:  "working directory must be a child of the current directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir, err := normalizeWorkingDir(tc.dir)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if got, want := dir, tc.exp; got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
		})
	}
}

func TestPlan_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		flagGitHubToken       string
		flagGitHubAction      bool
		flagGitHubOwner       string
		flagGitHubRepo        string
		flagPullRequestNumber int
		flagWorkingDirectory  string
		flagBucketName        string
		flagProtectLockfile   bool
		flagLockTimeout       time.Duration
		flagMaxRetries        uint64
		flagInitialRetryDelay time.Duration
		flagMaxRetryDelay     time.Duration
		config                *Config
		terraformClient       *terraform.MockTerraformClient
		err                   string
		expGitHubClientReqs   []*github.Request
		expStorageClientReqs  []*storage.Request
		expStdout             string
		expStderr             string
	}{
		{
			name:                  "success_with_diff",
			flagGitHubAction:      true,
			flagGitHubToken:       "github-token",
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 1,
			flagWorkingDirectory:  "../../../testdata",
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      int64(100),
				RunAttempt: int64(1),
			},
			terraformClient: &terraform.MockTerraformClient{
				InitResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform init success with diff",
					ExitCode: 0,
				},
				ValidateResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform validate success with diff",
					ExitCode: 0,
				},
				PlanResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform plan success with diff",
					ExitCode: 2,
				},
				ShowResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform show success with diff",
					ExitCode: 0,
				},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** - 🟩 Successful for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform show success with diff\n```\n</details>"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "UploadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/../../../testdata/test-tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                  "success_with_no_diff",
			flagGitHubAction:      true,
			flagGitHubToken:       "github-token",
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 2,
			flagWorkingDirectory:  "../../../testdata",
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      int64(100),
				RunAttempt: int64(1),
			},
			terraformClient: &terraform.MockTerraformClient{
				InitResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform init success",
					ExitCode: 0,
				},
				ValidateResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform validate success",
					ExitCode: 0,
				},
				PlanResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform plan success - no diff",
					ExitCode: 0,
				},
				ShowResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform show success - no diff",
					ExitCode: 0,
				},
			},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(2), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** - 🟦 No changes for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
			},
		},
		{
			name:                  "handles_error",
			flagGitHubAction:      true,
			flagGitHubToken:       "github-token",
			flagGitHubOwner:       "owner",
			flagGitHubRepo:        "repo",
			flagPullRequestNumber: 3,
			flagWorkingDirectory:  "../../../testdata",
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				ServerURL:  "https://github.com",
				RunID:      int64(100),
				RunAttempt: int64(1),
			},
			terraformClient: &terraform.MockTerraformClient{
				InitResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform init output",
					Stderr:   "terraform init failed",
					ExitCode: 1,
					Err:      fmt.Errorf("failed to run terraform init"),
				},
				ValidateResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform validate success",
					ExitCode: 0,
				},
				PlanResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform plan success - no diff",
					ExitCode: 0,
				},
				ShowResponse: &terraform.MockTerraformResponse{
					Stdout:   "terraform show success - no diff",
					ExitCode: 0,
				},
			},
			expStdout: "terraform init output",
			expStderr: "terraform init failed",
			err:       "failed to run Guardian plan: failed to initialize: failed to run terraform init",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(3), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name: "UpdateIssueComment",
					Params: []any{
						"owner",
						"repo",
						int64(1),
						"**`🔱 Guardian 🔱 PLAN`** - 🟥 Failed for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n" +
							"\n" +
							"<details>\n" +
							"<summary>Error</summary>\n" +
							"\n" +
							"```\n" +
							"\n" +
							"failed to initialize: failed to run terraform init\n" +
							"```\n" +
							"</details>\n" +
							"\n" +
							"<details>\n" +
							"<summary>Details</summary>\n" +
							"\n" +
							"```diff\n" +
							"\n" +
							"terraform init failed\n" +
							"```\n" +
							"</details>",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actions := githubactions.New(githubactions.WithWriter(os.Stdout))
			githubClient := &github.MockGitHubClient{}
			storageClient := &storage.MockStorageClient{}

			c := &PlanRunCommand{
				cfg: tc.config,

				planFilename: "test-tfplan.binary",

				flagWorkingDirectory:  tc.flagWorkingDirectory,
				flagPullRequestNumber: tc.flagPullRequestNumber,
				flagBucketName:        tc.flagBucketName,
				flagProtectLockfile:   tc.flagProtectLockfile,
				flagLockTimeout:       tc.flagLockTimeout,
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
				actions:         actions,
				githubClient:    githubClient,
				storageClient:   storageClient,
				terraformClient: tc.terraformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(githubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
				t.Errorf("GitHubClient calls not as expected; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(storageClient.Reqs, tc.expStorageClientReqs); diff != "" {
				t.Errorf("Storage calls not as expected; (-got,+want): %s", diff)
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
