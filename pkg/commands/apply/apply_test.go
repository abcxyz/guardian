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

package apply

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/commands/actions"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

var terraformMock = &terraform.MockTerraformClient{
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	ApplyResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform apply success",
		ExitCode: 0,
	},
}

var terraformErrorMock = &terraform.MockTerraformClient{
	InitResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform init success",
		ExitCode: 0,
	},
	ValidateResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform validate success",
		ExitCode: 0,
	},
	ApplyResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform apply output",
		Stderr:   "terraform apply failed",
		ExitCode: 1,
		Err:      fmt.Errorf("failed to run terraform apply"),
	},
}

func TestApply_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	defaultConfig := &Config{
		ServerURL:  "https://github.com",
		RunID:      int64(100),
		RunAttempt: int64(1),
	}

	cases := []struct {
		name                     string
		directory                string
		flagIsGitHubActions      bool
		flagGitHubOwner          string
		flagGitHubRepo           string
		flagCommitSHA            string
		flagPullRequestNumber    int
		flagBucketName           string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		flagJobName              string
		config                   *Config
		planExitCode             string
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expGitHubClientReqs      []*github.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
		resolveJobLogsURLErr     error
	}{
		{
			name:                     "success",
			directory:                "testdir",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagCommitSHA:            "commit-sha-1",
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagJobName:              "example-job",
			config:                   defaultConfig,
			planExitCode:             "2",
			terraformClient:          terraformMock,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListPullRequestsForCommit",
					Params: []any{"owner", "repo", "commit-sha-1"},
				},
				{
					Name:   "ResolveJobLogsURL",
					Params: []any{"example-job", "owner", "repo", int64(100)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 APPLY`** - 🟨 Running for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/job/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 APPLY`** - 🟩 Successful for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/job/1)]\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform apply success\n```\n</details>"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "ObjectMetadata",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DownloadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:                     "success_when_direct_log_url_resolution_fails",
			directory:                "testdir",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagCommitSHA:            "commit-sha-1",
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagJobName:              "example-job",
			config:                   defaultConfig,
			planExitCode:             "2",
			terraformClient:          terraformMock,
			resolveJobLogsURLErr:     fmt.Errorf("couldn't resolve job logs url"),
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ListPullRequestsForCommit",
					Params: []any{"owner", "repo", "commit-sha-1"},
				},
				{
					Name:   "ResolveJobLogsURL",
					Params: []any{"example-job", "owner", "repo", int64(100)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 APPLY`** - 🟨 Running for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 APPLY`** - 🟩 Successful for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform apply success\n```\n</details>"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "ObjectMetadata",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DownloadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:                     "skips_no_diff",
			directory:                "testdir",
			flagIsGitHubActions:      false,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    2,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagJobName:              "example-job",
			config:                   defaultConfig,
			planExitCode:             "0",
			terraformClient:          terraformMock,
			expStorageClientReqs: []*storage.Request{
				{
					Name: "ObjectMetadata",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/2/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DownloadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/2/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/2/testdir/test-tfplan.binary",
					},
				},
			},
			expStdout: "Guardian plan file has no diff, exiting",
		},
		{
			name:                     "handles_error",
			directory:                "testdir",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    3,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			flagJobName:              "example-job",
			config:                   defaultConfig,
			planExitCode:             "2",
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform apply output",
			expStderr:                "terraform apply failed",
			err:                      "failed to run Guardian apply: failed to apply: failed to run terraform apply",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "ResolveJobLogsURL",
					Params: []any{"example-job", "owner", "repo", int64(100)},
				},
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(3), "**`🔱 Guardian 🔱 APPLY`** - 🟨 Running for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/job/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 APPLY`** - 🟥 Failed for dir: `testdir` [[logs](https://github.com/owner/repo/actions/runs/100/job/1)]\n\n<details>\n<summary>Error</summary>\n\n```\n\nfailed to apply: failed to run terraform apply\n```\n</details>\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform apply failed\n```\n</details>"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "ObjectMetadata",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/3/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DownloadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/3/testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/3/testdir/test-tfplan.binary",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			action := githubactions.New(githubactions.WithWriter(os.Stdout))
			gitHubClient := &github.MockGitHubClient{
				ResolveJobLogsURLErr: tc.resolveJobLogsURLErr,
			}
			storageClient := &storage.MockStorageClient{
				Metadata: map[string]string{
					"plan_exit_code": tc.planExitCode,
				},
			}

			c := &ApplyCommand{
				GitHubActionCommand: actions.GitHubActionCommand{
					FlagIsGitHubActions: tc.flagIsGitHubActions,
					Action:              action,
				},
				GitHubFlags: flags.GitHubFlags{
					FlagGitHubOwner: tc.flagGitHubOwner,
					FlagGitHubRepo:  tc.flagGitHubRepo,
				},
				cfg: tc.config,

				directory:    tc.directory,
				childPath:    tc.directory,
				planFileName: "test-tfplan.binary",

				flagCommitSHA:            tc.flagCommitSHA,
				flagPullRequestNumber:    tc.flagPullRequestNumber,
				flagBucketName:           tc.flagBucketName,
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagLockTimeout:          tc.flagLockTimeout,
				flagJobName:              tc.flagJobName,
				gitHubClient:             gitHubClient,
				storageClient:            storageClient,
				terraformClient:          tc.terraformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(gitHubClient.Reqs, tc.expGitHubClientReqs); diff != "" {
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
