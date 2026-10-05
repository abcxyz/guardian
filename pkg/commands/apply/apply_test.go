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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/platform"
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

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                     string
		directory                string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		planExitCode             string
		storageParent            string
		storagePrefix            string
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expPlatformClientReqs    []*platform.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
		resolveJobLogsURLErr     error
	}{
		{
			name:      "success",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "2",
			terraformClient:          terraformMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusSuccess, &platform.StatusParams{HasDiff: true, Details: "terraform apply success", Dir: "testdir", Operation: "apply"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
		{
			name:      "skips_no_diff",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "0",
			terraformClient:          terraformMock,
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
			expStdout: "Guardian plan file has no diff, exiting",
		},
		{
			name:      "handles_error",
			directory: "testdir",

			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			planExitCode:             "2",
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform apply output",
			expStderr:                "terraform apply failed",
			err:                      "failed to run Guardian apply: failed to apply: failed to run terraform apply",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusFailure, &platform.StatusParams{HasDiff: true, Details: "terraform apply failed", Dir: "testdir", Operation: "apply"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "GetObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
				{
					Name: "DeleteObject",
					Params: []any{
						"testdir/test-tfplan.binary",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{
				Metadata: map[string]string{
					"plan_exit_code": tc.planExitCode,
				},
			}
			mockPlatformClient := &platform.MockPlatform{}

			c := &ApplyCommand{
				directory:                t.TempDir(),
				childPath:                tc.directory,
				planFilename:             "test-tfplan.binary",
				storagePrefix:            tc.storagePrefix,
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagLockTimeout:          tc.flagLockTimeout,
				storageClient:            mockStorageClient,
				terraformClient:          tc.terraformClient,
				platformClient:           mockPlatformClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if diff := cmp.Diff(mockPlatformClient.Reqs, tc.expPlatformClientReqs); diff != "" {
				t.Errorf("Platform calls not as expected; (-got,+want): %s", diff)
			}

			if diff := cmp.Diff(mockStorageClient.Reqs, tc.expStorageClientReqs); diff != "" {
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

func TestApply_Process_ProviderSources(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name           string
		allowedSources []string
		files          map[string]string
		installed      []string
		expErr         string
		expStatus      platform.Status
		expDetails     string
		expValidate    bool
	}{
		{
			name:           "allowed",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:      []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expStatus:      platform.StatusSuccess,
			expValidate:    true,
		},
		{
			// A pre-existing provider cache (e.g. from a previous local run, or
			// committed alongside a matching lock file) must not be reused. It is
			// removed before init, so only providers installed by init are checked.
			name:           "stale_provider_cache_removed_before_init",
			allowedSources: []string{"hashicorp/google"},
			files: map[string]string{
				"main.tf":                `resource "google_project" "p" {}`,
				".terraform/environment": "dev",
				".terraform/providers/registry.terraform.io/attacker/evil/1.0.0/linux_amd64/terraform-provider-evil": "#!/bin/sh",
			},
			installed:   []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expStatus:   platform.StatusSuccess,
			expValidate: true,
		},
		{
			name:           "terraform_d_rejected_before_init",
			allowedSources: []string{"hashicorp/google"},
			files: map[string]string{
				"main.tf":                 `resource "google_project" "p" {}`,
				"terraform.d/plugins/.ok": "",
			},
			expErr:     "failed provider source check: refusing to run: found terraform.d",
			expStatus:  platform.StatusFailure,
			expDetails: "found terraform.d",
		},
		{
			name:           "declared_source_rejected_before_init",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `data "external" "x" {}`},
			expErr:         "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/hashicorp/external",
			expStatus:      platform.StatusFailure,
			expDetails:     "registry.terraform.io/hashicorp/external",
		},
		{
			name:           "installed_source_rejected_before_validate",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				"registry.terraform.io/attacker/evil/1.0.0/linux_amd64",
			},
			expErr:     "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/attacker/evil",
			expStatus:  platform.StatusFailure,
			expDetails: "registry.terraform.io/attacker/evil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tc.files {
				p := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			tfClient := &terraform.MockTerraformClient{
				InitHook: func() error {
					for _, p := range tc.installed {
						if err := os.MkdirAll(filepath.Join(dir, ".terraform", "providers", p), 0o755); err != nil {
							return err
						}
					}
					return nil
				},
				ValidateResponse: &terraform.MockTerraformResponse{Stdout: "VALIDATE_CALLED"},
				ApplyResponse:    &terraform.MockTerraformResponse{Stdout: "terraform apply success"},
			}

			mockPlatformClient := &platform.MockPlatform{}
			c := &ApplyCommand{
				directory:       dir,
				childPath:       "dir",
				planFilename:    "test-tfplan.binary",
				flagLockTimeout: 10 * time.Minute,
				storageClient: &storage.MockStorageClient{
					Metadata: map[string]string{"plan_exit_code": "2"},
				},
				terraformClient: tfClient,
				platformClient:  mockPlatformClient,
			}
			c.FlagAllowedProviderSources = tc.allowedSources

			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}

			if got := strings.Contains(stdout.String(), "VALIDATE_CALLED"); got != tc.expValidate {
				t.Errorf("validate called = %t, want %t", got, tc.expValidate)
			}

			if len(mockPlatformClient.Reqs) != 1 {
				t.Fatalf("expected 1 platform request, got %d", len(mockPlatformClient.Reqs))
			}
			params := mockPlatformClient.Reqs[0].Params
			if got := params[0]; got != tc.expStatus {
				t.Errorf("status = %v, want %v", got, tc.expStatus)
			}
			sp, ok := params[1].(*platform.StatusParams)
			if !ok {
				t.Fatalf("expected *platform.StatusParams, got %T", params[1])
			}
			if !strings.Contains(sp.Details, tc.expDetails) {
				t.Errorf("status details %q do not contain %q", sp.Details, tc.expDetails)
			}
		})
	}
}
