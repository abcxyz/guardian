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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestPlan_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cases := []struct {
		name                  string
		flagGitHubToken       string
		flagConcurrency       int64
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
			flagGitHubToken:       "github-token",
			flagWorkingDirectory:  "../../../testdata",
			flagConcurrency:       2,
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				IsAction:          true,
				EventName:         "pull_request_target",
				RepositoryOwner:   "owner",
				RepositoryName:    "repo",
				PullRequestNumber: 1,
				ServerURL:         "https://github.com",
				RunID:             int64(100),
				RunAttempt:        int64(1),
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
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", int(1)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** -  🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** -  🟩 Successful for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform show success with diff\n```\n</details>"},
				},
				{
					Name:   "DeleteIssueComment",
					Params: []any{"owner", "repo", int64(1)},
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
			flagGitHubToken:       "github-token",
			flagWorkingDirectory:  "../../../testdata",
			flagConcurrency:       2,
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				IsAction:          true,
				EventName:         "pull_request_target",
				RepositoryOwner:   "owner",
				RepositoryName:    "repo",
				PullRequestNumber: 1,
				ServerURL:         "https://github.com",
				RunID:             int64(100),
				RunAttempt:        int64(1),
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
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", int(1)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** -  🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** -  🟦 No Terraform files have changes, planning skipped."},
				},
			},
		},
		{
			name:                  "success_no_changes",
			flagGitHubToken:       "github-token",
			flagWorkingDirectory:  "../../../testdata",
			flagConcurrency:       2,
			flagBucketName:        "my-bucket-name",
			flagProtectLockfile:   true,
			flagLockTimeout:       10 * time.Minute,
			flagMaxRetries:        3,
			flagInitialRetryDelay: 2 * time.Second,
			flagMaxRetryDelay:     10 * time.Second,
			config: &Config{
				IsAction:          true,
				EventName:         "pull_request_target",
				RepositoryOwner:   "owner",
				RepositoryName:    "repo",
				PullRequestNumber: 1,
				ServerURL:         "https://github.com",
				RunID:             int64(100),
				RunAttempt:        int64(1),
			},
			terraformClient: &terraform.MockTerraformClient{},
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListIssueComments",
					Params: []any{"owner", "repo", int(1)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** -  🟨 Running for dir: `../../../testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** -  🟦 No Terraform files have changes, planning skipped."},
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

			c := &PlanCommand{
				cfg: tc.config,

				planFilename: "test-tfplan.binary",

				flagGitHubToken:       tc.flagGitHubToken,
				flagConcurrency:       tc.flagConcurrency,
				flagWorkingDirectory:  tc.flagWorkingDirectory,
				flagBucketName:        tc.flagBucketName,
				flagProtectLockfile:   tc.flagProtectLockfile,
				flagLockTimeout:       tc.flagLockTimeout,
				flagMaxRetries:        tc.flagMaxRetries,
				flagInitialRetryDelay: tc.flagInitialRetryDelay,
				flagMaxRetryDelay:     tc.flagMaxRetryDelay,

				actions:         actions,
				githubClient:    githubClient,
				storageClient:   storageClient,
				terraformClient: tc.terraformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf("unexpected err: %s", diff)
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
