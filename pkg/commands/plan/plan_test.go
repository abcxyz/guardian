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
	"unicode/utf8"

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

func TestAfterParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "validate_github_flags",
			args: []string{"-pull-request-number=1", "-bucket-name=my-bucket"},
			err:  "missing flag: github-owner is required\nmissing flag: github-repo is required",
		},
		{
			name: "validate_pull_request_number",
			args: []string{"-github-owner=owner", "-github-repo=repo", "-bucket-name=my-bucket"},
			err:  "missing flag: pull-request-number is required",
		},
		{
			name: "validate_bucket_name",
			args: []string{"-github-owner=owner", "-github-repo=repo", "-pull-request-number=1"},
			err:  "missing flag: bucket-name is required",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &PlanCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

var terraformNoDiffMock = &terraform.MockTerraformClient{
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
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
}

var terraformDiffMock = &terraform.MockTerraformClient{
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
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
}

var terraformErrorMock = &terraform.MockTerraformClient{
	FormatResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform format success",
		ExitCode: 0,
	},
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
}

func TestPlan_Process(t *testing.T) {
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
		flagPullRequestNumber    int
		flagBucketName           string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		config                   *Config
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expGitHubClientReqs      []*github.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
	}{
		{
			name:                     "success_with_diff",
			directory:                "testdata",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    1,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			config:                   defaultConfig,
			terraformClient:          terraformDiffMock,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(1), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** - 🟩 Successful for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nterraform show success with diff\n```\n</details>"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "UploadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/1/testdata/test-tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "success_with_no_diff",
			directory:                "testdata",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    2,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			config:                   defaultConfig,
			terraformClient:          terraformNoDiffMock,
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(2), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name:   "UpdateIssueComment",
					Params: []any{"owner", "repo", int64(1), "**`🔱 Guardian 🔱 PLAN`** - 🟦 No changes for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "UploadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/2/testdata/test-tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "skips_comments",
			directory:                "testdata",
			flagIsGitHubActions:      false,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    2,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			config:                   defaultConfig,
			terraformClient:          terraformNoDiffMock,
			expStorageClientReqs: []*storage.Request{
				{
					Name: "UploadObject",
					Params: []any{
						"my-bucket-name",
						"guardian-plans/owner/repo/2/testdata/test-tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "handles_error",
			directory:                "testdata",
			flagIsGitHubActions:      true,
			flagGitHubOwner:          "owner",
			flagGitHubRepo:           "repo",
			flagPullRequestNumber:    3,
			flagBucketName:           "my-bucket-name",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			config:                   defaultConfig,
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform init output",
			expStderr:                "terraform init failed",
			err:                      "failed to run Guardian plan: failed to initialize: failed to run terraform init",
			expGitHubClientReqs: []*github.Request{
				{
					Name:   "CreateIssueComment",
					Params: []any{"owner", "repo", int(3), "**`🔱 Guardian 🔱 PLAN`** - 🟨 Running for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]"},
				},
				{
					Name: "UpdateIssueComment",
					Params: []any{
						"owner",
						"repo",
						int64(1),
						"**`🔱 Guardian 🔱 PLAN`** - 🟥 Failed for dir: `testdata` [[logs](https://github.com/owner/repo/actions/runs/100/attempts/1)]\n" +
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

			action := githubactions.New(githubactions.WithWriter(os.Stdout))
			gitHubClient := &github.MockGitHubClient{}
			storageClient := &storage.MockStorageClient{}

			c := &PlanCommand{
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
				planFilename: "test-tfplan.binary",

				flagPullRequestNumber:    tc.flagPullRequestNumber,
				flagBucketName:           tc.flagBucketName,
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagLockTimeout:          tc.flagLockTimeout,
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

func TestGetMessageBody(t *testing.T) {
	t.Parallel()
	bigMessage := messageOverLimit()
	cases := []struct {
		name      string
		cmd       *PlanCommand
		result    *RunResult
		resultErr error
		want      string
	}{
		{
			name: "result_success",
			cmd: &PlanCommand{
				childPath:    "foo",
				gitHubLogURL: "http://github.com/logs",
			},
			result: &RunResult{
				hasChanges:     true,
				commentDetails: "This comment is within the limits",
			},
			resultErr: nil,
			want:      "**`🔱 Guardian 🔱 PLAN`** - 🟩 Successful for dir: `foo` http://github.com/logs\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nThis comment is within the limits\n```\n</details>",
		},
		{
			name: "result_error",
			cmd: &PlanCommand{
				childPath:    "foo",
				gitHubLogURL: "http://github.com/logs",
			},
			result: &RunResult{
				hasChanges:     true,
				commentDetails: "This is a detailed error message",
			},
			resultErr: fmt.Errorf("the result had an error"),
			want:      "**`🔱 Guardian 🔱 PLAN`** - 🟥 Failed for dir: `foo` http://github.com/logs\n\n<details>\n<summary>Error</summary>\n\n```\n\nthe result had an error\n```\n</details>\n\n<details>\n<summary>Details</summary>\n\n```diff\n\nThis is a detailed error message\n```\n</details>",
		},
		{
			name: "result_no_changes",
			cmd: &PlanCommand{
				childPath:    "foo",
				gitHubLogURL: "http://github.com/logs",
			},
			result: &RunResult{
				hasChanges:     false,
				commentDetails: "",
			},
			resultErr: nil,
			want:      "**`🔱 Guardian 🔱 PLAN`** - 🟦 No changes for dir: `foo` http://github.com/logs",
		},
		{
			name: "result_success_over_limit",
			cmd: &PlanCommand{
				childPath:    "foo",
				gitHubLogURL: "http://github.com/logs",
			},
			result: &RunResult{
				hasChanges:     true,
				commentDetails: bigMessage,
			},
			resultErr: nil,
			want:      fmt.Sprintf("**`🔱 Guardian 🔱 PLAN`** - 🟩 Successful for dir: `foo` http://github.com/logs\n\n<details>\n<summary>Details</summary>\n\n```diff\n\n%s...\n```\n</details>\n\nMessage has been truncated. See workflow logs to view the full message.", bigMessage[:gitHubMaxCommentLength-216]),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.cmd.getMessageBody(tc.result, tc.resultErr)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}
			if length := utf8.RuneCountInString(got); length > gitHubMaxCommentLength {
				t.Errorf("message produced had length %d over the maximum length: %s", length, got)
			}
		})
	}
}

func messageOverLimit() string {
	message := make([]rune, gitHubMaxCommentLength)
	for i := 0; i < len(message); i++ {
		message[i] = 'a'
	}
	return string(message)
}
