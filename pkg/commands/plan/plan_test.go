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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/checkterraform/plugintest"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

var terraformNoDiffMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
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
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success - no diff"}`,
		ExitCode: 0,
	},
}

var terraformDiffMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
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
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success with diff"}`,
		ExitCode: 0,
	},
}

var terraformErrorMock = &terraform.MockTerraformClient{
	PlanBody: []byte("this is a plan binary"),
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
	ShowJSONResponse: &terraform.MockTerraformResponse{
		Stdout:   `{"result": "terraform show success - no diff"}`,
		ExitCode: 0,
	},
}

func TestPlan_Process(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name                     string
		directory                string
		storageParent            string
		storagePrefix            string
		flagAllowLockfileChanges bool
		flagLockTimeout          time.Duration
		flagReportStdout         bool
		terraformClient          *terraform.MockTerraformClient
		err                      string
		expPlatformClientReqs    []*platform.Request
		expStorageClientReqs     []*storage.Request
		expStdout                string
		expStderr                string
	}{
		{
			name:                     "success_with_diff",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformDiffMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusSuccess, &platform.StatusParams{HasDiff: true, Details: "terraform show success with diff", Dir: "testdata", Operation: "plan"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "CreateObject",
					Params: []any{
						"testdata/tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "success_with_diff_and_report_stdout",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagReportStdout:         true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformDiffMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusSuccess, &platform.StatusParams{HasDiff: true, Details: "terraform plan success with diff", Dir: "testdata", Operation: "plan"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "CreateObject",
					Params: []any{
						"testdata/tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "success_with_no_diff",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformNoDiffMock,
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusNoOperation, &platform.StatusParams{HasDiff: false, Dir: "testdata", Operation: "plan"}},
				},
			},
			expStorageClientReqs: []*storage.Request{
				{
					Name: "CreateObject",
					Params: []any{
						"testdata/tfplan.binary",
						"this is a plan binary",
					},
				},
			},
		},
		{
			name:                     "handles_error",
			directory:                "testdata",
			storagePrefix:            "",
			flagAllowLockfileChanges: true,
			flagLockTimeout:          10 * time.Minute,
			terraformClient:          terraformErrorMock,
			expStdout:                "terraform init output",
			expStderr:                "terraform init failed",
			err:                      "failed to run Guardian plan: failed to initialize: failed to run terraform init",
			expPlatformClientReqs: []*platform.Request{
				{
					Name:   "Status",
					Params: []any{platform.StatusFailure, &platform.StatusParams{HasDiff: false, Details: "terraform init failed", ErrorMessage: "failed to initialize: failed to run terraform init", Dir: "testdata", Operation: "plan"}},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockStorageClient := &storage.MockStorageClient{}
			mockPlatformClient := &platform.MockPlatform{}

			c := &PlanCommand{
				directory:                tc.directory,
				childPath:                tc.directory,
				storagePrefix:            tc.storagePrefix,
				flagOutputDir:            t.TempDir(),
				flagAllowLockfileChanges: tc.flagAllowLockfileChanges,
				flagReportStdout:         tc.flagReportStdout,
				flagLockTimeout:          tc.flagLockTimeout,
				terraformClient:          tc.terraformClient,
				storageClient:            mockStorageClient,
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

func TestPlan_Process_ProviderSources(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name             string
		allowedSources   []string
		files            map[string]string
		installed        []string
		expErr           string
		expStatus        platform.Status
		expDetails       string
		expValidateCalls bool
	}{
		{
			name:             "allowed",
			allowedSources:   []string{"hashicorp/google"},
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:        []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expStatus:        platform.StatusSuccess,
			expValidateCalls: true,
		},
		{
			name:           "invalid_pattern",
			allowedSources: []string{"!hashicorp/external"},
			expErr:         "invalid -allowed-provider-sources",
			expStatus:      platform.StatusFailure,
			expDetails:     "exclusions are not supported",
		},
		{
			name:           "local_override_rejected_before_init",
			allowedSources: []string{"hashicorp/google"},
			files: map[string]string{
				"main.tf": `resource "google_project" "p" {}`,
				"terraform.d/plugins/registry.terraform.io/hashicorp/google/9.9.9/linux_amd64/terraform-provider-google": "#!/bin/sh",
			},
			expErr:     "failed local plugin check: refusing to run: found terraform.d",
			expStatus:  platform.StatusFailure,
			expDetails: "found terraform.d",
		},
		{
			// e.g. re-running guardian plan locally in an already-initialized
			// directory. The provider cache is removed and re-installed by init.
			name:           "local_rerun_with_existing_dot_terraform",
			allowedSources: []string{"hashicorp/google"},
			files: map[string]string{
				"main.tf":                         `resource "google_project" "p" {}`,
				".terraform/environment":          "dev",
				".terraform/terraform.tfstate":    "{}",
				".terraform/modules/modules.json": `{"Modules":[]}`,
				".terraform/providers/registry.terraform.io/attacker/evil/1.0.0/linux_amd64/terraform-provider-evil": "#!/bin/sh",
			},
			installed:        []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expStatus:        platform.StatusSuccess,
			expValidateCalls: true,
		},
		{
			name:           "declared_source_rejected_before_init",
			allowedSources: []string{"hashicorp/*"},
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil = { source = "attacker/evil" }
  }
}
`},
			expErr:     "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/attacker/evil",
			expStatus:  platform.StatusFailure,
			expDetails: "registry.terraform.io/attacker/evil",
		},
		{
			// e.g. a provider introduced by a remote module that the static check
			// cannot see before init.
			name:           "installed_source_rejected_before_validate",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				"registry.terraform.io/hashicorp/external/2.0.0/linux_amd64",
			},
			expErr:     "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/hashicorp/external",
			expStatus:  platform.StatusFailure,
			expDetails: "registry.terraform.io/hashicorp/external",
		},
		{
			name:           "empty_denies_all_sources",
			allowedSources: nil,
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:      []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expErr:         "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/hashicorp/google (no provider sources are allowed",
			expStatus:      platform.StatusFailure,
			expDetails:     "registry.terraform.io/hashicorp/google",
		},
		{
			name:             "wildcard_allows_all_sources",
			allowedSources:   []string{"*"},
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:        []string{"registry.terraform.io/attacker/evil/1.0.0/linux_amd64"},
			expStatus:        platform.StatusSuccess,
			expValidateCalls: true,
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
				PlanBody: []byte("this is a plan binary"),
				InitHook: func() error {
					for _, p := range tc.installed {
						if err := os.MkdirAll(filepath.Join(dir, ".terraform", "providers", p), 0o755); err != nil {
							return err
						}
					}
					return nil
				},
				ValidateResponse: &terraform.MockTerraformResponse{Stdout: "VALIDATE_CALLED"},
				PlanResponse:     &terraform.MockTerraformResponse{ExitCode: 2},
				ShowResponse:     &terraform.MockTerraformResponse{Stdout: "terraform show success with diff"},
			}

			mockPlatformClient := &platform.MockPlatform{}
			c := &PlanCommand{
				directory:       dir,
				childPath:       "dir",
				flagOutputDir:   t.TempDir(),
				flagLockTimeout: 10 * time.Minute,
				terraformClient: tfClient,
				storageClient:   &storage.MockStorageClient{},
				platformClient:  mockPlatformClient,
			}
			c.FlagAllowedProviderSources = tc.allowedSources

			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}

			if got := strings.Contains(stdout.String(), "VALIDATE_CALLED"); got != tc.expValidateCalls {
				t.Errorf("validate called = %t, want %t", got, tc.expValidateCalls)
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

func TestPlan_Process_LocalProvisionerPlugins(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	for _, tc := range plugintest.Cases() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tc.Setup(t, dir, filepath.Join(dir, "marker"))

			var initCalled bool
			tfClient := &terraform.MockTerraformClient{
				PlanBody: []byte("this is a plan binary"),
				InitHook: func() error {
					initCalled = true
					return tc.CheckRemoved(dir)
				},
				ValidateResponse: &terraform.MockTerraformResponse{Stdout: "VALIDATE_CALLED"},
				PlanResponse:     &terraform.MockTerraformResponse{ExitCode: 2},
				ShowResponse:     &terraform.MockTerraformResponse{Stdout: "terraform show success with diff"},
			}

			c := &PlanCommand{
				directory:       dir,
				childPath:       "dir",
				flagOutputDir:   t.TempDir(),
				flagLockTimeout: 10 * time.Minute,
				terraformClient: tfClient,
				storageClient:   &storage.MockStorageClient{},
				platformClient:  &platform.MockPlatform{},
			}
			c.FlagAllowedProviderSources = []string{"*"}
			// Plugins must be rejected even when every provisioner is allowed.
			c.FlagAllowedProvisionerSources = []string{"*"}

			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)

			if tc.WantRejected {
				if diff := testutil.DiffErrString(err, plugintest.RejectedErr); diff != "" {
					t.Error(diff)
				}
				if initCalled {
					t.Errorf("terraform init was called")
				}
				if strings.Contains(stdout.String(), "VALIDATE_CALLED") {
					t.Errorf("terraform was executed after init")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !initCalled {
				t.Errorf("terraform init was not called")
			}
			if !strings.Contains(stdout.String(), "VALIDATE_CALLED") {
				t.Errorf("terraform was not executed after init")
			}
		})
	}
}

func TestPlan_Process_ProvisionerSources(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	const localExec = `resource "terraform_data" "x" {
  provisioner "local-exec" {
    command = "true"
  }
}
`

	cases := []struct {
		name                string
		allowedProvisioners []string
		files               map[string]string
		// moduleFiles are written by terraform init, e.g. a remote module.
		moduleFiles map[string]string
		expErr      string
		expInit     bool
		expExecuted bool
	}{
		{
			name:        "no_provisioners",
			files:       map[string]string{"main.tf": `resource "terraform_data" "x" {}`},
			expInit:     true,
			expExecuted: true,
		},
		{
			name:   "denied_by_default_before_init",
			files:  map[string]string{"main.tf": localExec},
			expErr: "failed provisioner source check: terraform uses provisioners that are not allowed: builtin/local-exec (no provisioners are allowed",
		},
		{
			name:                "allowed",
			allowedProvisioners: []string{"builtin/local-exec"},
			files:               map[string]string{"main.tf": localExec},
			expInit:             true,
			expExecuted:         true,
		},
		{
			name:                "plugin_denied",
			allowedProvisioners: []string{"builtin/*"},
			files: map[string]string{"main.tf": `resource "terraform_data" "x" {
  provisioner "custom" {}
}
`},
			expErr: "not allowed: plugin/custom",
		},
		{
			name:  "module_installed_by_init_denied_before_execution",
			files: map[string]string{"main.tf": `module "m" { source = "example.com/acme/m/x" }`},
			moduleFiles: map[string]string{
				".terraform/modules/modules.json": `{"Modules":[{"Key":"","Source":"","Dir":"."},{"Key":"m","Source":"example.com/acme/m/x","Dir":".terraform/modules/m"}]}`,
				".terraform/modules/m/main.tf":    localExec,
			},
			expErr:  "failed provisioner source check: terraform uses provisioners that are not allowed: builtin/local-exec",
			expInit: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, filepath.Join(dir, name), content)
			}

			var initCalled bool
			tfClient := &terraform.MockTerraformClient{
				PlanBody:         []byte("this is a plan binary"),
				ValidateResponse: &terraform.MockTerraformResponse{Stdout: "VALIDATE_CALLED"},
				PlanResponse:     &terraform.MockTerraformResponse{ExitCode: 2},
				ShowResponse:     &terraform.MockTerraformResponse{Stdout: "terraform show success with diff"},
			}
			tfClient.InitHook = func() error {
				initCalled = true
				for name, content := range tc.moduleFiles {
					p := filepath.Join(dir, name)
					if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
						return err
					}
				}
				return nil
			}

			c := &PlanCommand{
				directory:       dir,
				childPath:       "dir",
				flagOutputDir:   t.TempDir(),
				flagLockTimeout: 10 * time.Minute,
				terraformClient: tfClient,
				storageClient:   &storage.MockStorageClient{},
				platformClient:  &platform.MockPlatform{},
			}
			c.FlagAllowedProviderSources = []string{"*"}
			c.FlagAllowedProvisionerSources = tc.allowedProvisioners

			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			if initCalled != tc.expInit {
				t.Errorf("terraform init called = %t, want %t", initCalled, tc.expInit)
			}
			if got := strings.Contains(stdout.String(), "VALIDATE_CALLED"); got != tc.expExecuted {
				t.Errorf("terraform executed after init = %t, want %t", got, tc.expExecuted)
			}
		})
	}
}

func writeFile(tb testing.TB, p, content string) {
	tb.Helper()

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		tb.Fatal(err)
	}
}
