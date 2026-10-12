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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abcxyz/guardian/pkg/checkterraform/plugintest"
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

	testDir := t.TempDir()
	writeTestFile(t, testDir, "disallowed_provider.tf", `resource "disallowed_provider" "example" {}`)

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
		flagDisallowedProviders      []string
		flagDisallowedProvisioners   []string
		flagAllowedProviders         []string
		flagAllowedProvisioners      []string
		terraformClient              *terraform.MockTerraformClient
		err                          string
		expStdout                    string
		expStderr                    string
	}{
		{
			name:                         "success",
			directory:                    testDir,
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
			directory:                    testDir,
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
			directory:                    testDir,
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
		{
			name:                         "calls_provider_check",
			directory:                    testDir,
			flagIsGitHubActions:          true,
			flagGitHubOwner:              "owner",
			flagGitHubRepo:               "repo",
			flagAllowedTerraformCommands: []string{},
			flagTerraformCommand:         "apply",
			flagTerraformArgs:            []string{"-no-color", "-input=false"},
			flagAllowLockfileChanges:     true,
			flagLockTimeout:              10 * time.Minute,
			flagDisallowedProviders:      []string{"disallowed"},
			terraformClient:              terraformMock,
			err:                          "terraform provider/provisioner check failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &RunCommand{
				directory: tc.directory,
				childPath: "testdir",

				flagAllowedTerraformCommands: tc.flagAllowedTerraformCommands,
				terraformCommand:             tc.flagTerraformCommand,
				terraformArgs:                tc.flagTerraformArgs,
				flagAllowLockfileChanges:     tc.flagAllowLockfileChanges,
				flagLockTimeout:              tc.flagLockTimeout,
				flagDisallowedProviders:      tc.flagDisallowedProviders,
				flagDisallowedProvisioners:   tc.flagDisallowedProvisioners,
				flagAllowedProviders:         tc.flagAllowedProviders,
				flagAllowedProvisioners:      tc.flagAllowedProvisioners,
				terraformClient:              tc.terraformClient,
			}
			c.FlagAllowedProviderSources = []string{"*"}

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

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func TestRun_Process_ProviderSources(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cases := []struct {
		name           string
		command        string
		allowedSources []string
		files          map[string]string
		installed      []string
		expErr         string
		expRun         bool
	}{
		{
			name:           "allowed",
			command:        "plan",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:      []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			expRun:         true,
		},
		{
			name:           "invalid_pattern",
			command:        "plan",
			allowedSources: []string{"a/b/c/d"},
			expErr:         "invalid -allowed-provider-sources",
		},
		{
			name:           "local_override_rejected_before_init",
			command:        "plan",
			allowedSources: []string{"hashicorp/google"},
			files: map[string]string{
				"main.tf":                 `resource "google_project" "p" {}`,
				"terraform.d/plugins/.ok": "",
			},
			expErr: "failed local plugin check: refusing to run: found terraform.d",
		},
		{
			name:           "declared_source_rejected_before_init",
			command:        "plan",
			allowedSources: []string{"hashicorp/*"},
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil = { source = "attacker/evil" }
  }
}
`},
			expErr: "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/attacker/evil",
		},
		{
			name:           "installed_source_rejected_before_command",
			command:        "apply",
			allowedSources: []string{"hashicorp/google"},
			files:          map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				"registry.terraform.io/attacker/evil/1.0.0/linux_amd64",
			},
			expErr: "failed provider source check: terraform uses provider sources that are not allowed: registry.terraform.io/attacker/evil",
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
				writeTestFile(t, filepath.Dir(p), filepath.Base(p), content)
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
				RunResponse: &terraform.MockTerraformResponse{Stdout: "RUN_CALLED"},
			}

			c := &RunCommand{
				directory:        dir,
				childPath:        "testdir",
				terraformCommand: tc.command,
				flagLockTimeout:  10 * time.Minute,
				terraformClient:  tfClient,
			}
			c.FlagAllowedProviderSources = tc.allowedSources

			_, stdout, _ := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			if got := strings.Contains(stdout.String(), "RUN_CALLED"); got != tc.expRun {
				t.Errorf("terraform command run = %t, want %t", got, tc.expRun)
			}
		})
	}
}

func TestRun_Process_LocalProvisionerPlugins(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	for _, tc := range plugintest.Cases() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tc.Setup(t, dir, filepath.Join(dir, "marker"))

			var initCalled bool
			tfClient := &terraform.MockTerraformClient{
				InitHook: func() error {
					initCalled = true
					return tc.CheckRemoved(dir)
				},
				RunResponse: &terraform.MockTerraformResponse{Stdout: "RUN_CALLED"},
			}

			c := &RunCommand{
				directory:        dir,
				childPath:        "testdir",
				terraformCommand: "plan",
				flagLockTimeout:  10 * time.Minute,
				terraformClient:  tfClient,
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
				if strings.Contains(stdout.String(), "RUN_CALLED") {
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
			if !strings.Contains(stdout.String(), "RUN_CALLED") {
				t.Errorf("terraform was not executed after init")
			}
		})
	}
}

func TestRun_Process_ProvisionerSources(t *testing.T) {
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
				RunResponse: &terraform.MockTerraformResponse{Stdout: "RUN_CALLED"},
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

			c := &RunCommand{
				directory:        dir,
				childPath:        "testdir",
				terraformCommand: "plan",
				flagLockTimeout:  10 * time.Minute,
				terraformClient:  tfClient,
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
			if got := strings.Contains(stdout.String(), "RUN_CALLED"); got != tc.expExecuted {
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
