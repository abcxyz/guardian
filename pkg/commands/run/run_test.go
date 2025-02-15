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

package run

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

var terraformMock = &terraform.MockTerraformClient{
	RunResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform run success",
		ExitCode: 0,
	},
}

var terraformErrorMock = &terraform.MockTerraformClient{
	RunResponse: &terraform.MockTerraformResponse{
		Stdout:   "terraform run output",
		Stderr:   "terraform run failed",
		ExitCode: 1,
		Err:      fmt.Errorf("failed to run terraform run"),
	},
}

func TestPlan_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                         string
		directory                    string
		flagIsGitHubActions          bool
		flagGitHubOwner              string
		flagGitHubRepo               string
		flagAllowedTerraformCommands []string
		flagTerraformCommand         string
		flagTerraformArgs            []string
		flagAllowLockfileChanges     bool
		flagLockTimeout              time.Duration
		terraformClient              *terraform.MockTerraformClient
		err                          string
		expStdout                    string
		expStderr                    string
	}{
		{
			name:                         "success",
			directory:                    "testdata",
			flagIsGitHubActions:          true,
			flagGitHubOwner:              "owner",
			flagGitHubRepo:               "repo",
			flagAllowedTerraformCommands: []string{},
			flagTerraformCommand:         "apply",
			flagTerraformArgs:            []string{"-no-color", "-input=false"},
			flagAllowLockfileChanges:     true,
			flagLockTimeout:              10 * time.Minute,
			terraformClient:              terraformMock,
		},
		{
			name:                         "retricts_allowed_commands",
			directory:                    "testdata",
			flagIsGitHubActions:          true,
			flagGitHubOwner:              "owner",
			flagGitHubRepo:               "repo",
			flagAllowedTerraformCommands: []string{"plan"},
			flagTerraformCommand:         "apply",
			flagTerraformArgs:            []string{"-no-color", "-input=false"},
			flagAllowLockfileChanges:     true,
			flagLockTimeout:              10 * time.Minute,
			terraformClient:              terraformMock,
			err:                          "apply is not an allowed Terraform command.\n\nAllowed commands are [\"plan\"]",
		},
		{
			name:                         "handles_errors",
			directory:                    "testdata",
			flagIsGitHubActions:          true,
			flagGitHubOwner:              "owner",
			flagGitHubRepo:               "repo",
			flagAllowedTerraformCommands: []string{},
			flagTerraformCommand:         "apply",
			flagTerraformArgs:            []string{"-no-color", "-input=false"},
			flagAllowLockfileChanges:     true,
			flagLockTimeout:              10 * time.Minute,
			terraformClient:              terraformErrorMock,
			expStdout:                    "terraform run output",
			expStderr:                    "terraform run failed",
			err:                          "failed to run command: failed to run terraform run",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &RunCommand{
				directory: "testdir",
				childPath: "testdir",

				flagAllowedTerraformCommands: tc.flagAllowedTerraformCommands,
				terraformCommand:             tc.flagTerraformCommand,
				terraformArgs:                tc.flagTerraformArgs,
				flagAllowLockfileChanges:     tc.flagAllowLockfileChanges,
				flagLockTimeout:              tc.flagLockTimeout,
				terraformClient:              tc.terraformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
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
