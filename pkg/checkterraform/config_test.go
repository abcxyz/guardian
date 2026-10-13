// Copyright 2026 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkterraform

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestCheckConfiguration_Providers(t *testing.T) {
	t.Parallel()

	hashicorp := func(typ string) *ProviderSource {
		return &ProviderSource{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: typ}
	}

	cases := []struct {
		name        string
		files       map[string]string
		patterns    []string
		expDeclared []*ProviderSource
		expErr      string
	}{
		{
			name:        "empty",
			files:       map[string]string{"main.tf": ""},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{},
		},
		{
			name: "required_providers_allowed",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    google-beta = {
      source                = "hashicorp/google-beta"
      configuration_aliases = [google-beta.alt]
    }
  }
}

resource "google_project" "p" {}
`},
			patterns:    []string{"hashicorp/google", "hashicorp/google-beta"},
			expDeclared: []*ProviderSource{hashicorp("google"), hashicorp("google-beta")},
		},
		{
			name: "attacker_source_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil = {
      source = "attacker/evil"
    }
  }
}
`},
			patterns: []string{"hashicorp/*"},
			expDeclared: []*ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "evil"},
			},
			expErr: "not allowed: registry.terraform.io/attacker/evil",
		},
		{
			name: "aliased_local_name_resolves_to_source",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    foo = {
      source = "hashicorp/external"
    }
  }
}

data "external" "x" {
  provider = foo
  program  = ["sh", "-c", "echo {}"]
}
`},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{hashicorp("external")},
			expErr:      "not allowed: registry.terraform.io/hashicorp/external",
		},
		{
			name: "implied_provider_from_resource_type",
			files: map[string]string{"main.tf": `
data "external" "x" {
  program = ["sh", "-c", "echo {}"]
}
`},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{hashicorp("external")},
			expErr:      "not allowed: registry.terraform.io/hashicorp/external",
		},
		{
			name: "provider_meta_argument_overrides_type_prefix",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google-beta = {
      source = "hashicorp/google-beta"
    }
  }
}

resource "google_project" "p" {
  provider = google-beta.alt
}
`},
			patterns:    []string{"hashicorp/google-beta"},
			expDeclared: []*ProviderSource{hashicorp("google-beta")},
		},
		{
			name: "builtin_terraform_provider_skipped",
			files: map[string]string{"main.tf": `
data "terraform_remote_state" "s" {}
resource "terraform_data" "d" {}
`},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{},
		},
		{
			name: "legacy_version_string",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = "~> 6.0"
  }
}
`},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{hashicorp("google")},
		},
		{
			name: "json_config",
			files: map[string]string{"main.tf.json": `{
  "terraform": {
    "required_providers": {
      "evil": { "source": "attacker/evil" }
    }
  }
}`},
			patterns: []string{"hashicorp/*"},
			expDeclared: []*ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "evil"},
			},
			expErr: "not allowed: registry.terraform.io/attacker/evil",
		},
		{
			name: "module_dirs_resolved_independently",
			files: map[string]string{
				"main.tf": `
terraform {
  required_providers {
    thing = { source = "hashicorp/google" }
  }
}
`,
				// The child module uses the same local name for a different source.
				"modules/child/main.tf": `
terraform {
  required_providers {
    thing = { source = "attacker/thing" }
  }
}
`,
				".terraform/modules/modules.json": `{"Modules":[{"Key":"","Source":"","Dir":"."},{"Key":"child","Source":"./modules/child","Dir":"modules/child"}]}`,
			},
			patterns: []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "thing"},
				hashicorp("google"),
			},
			expErr: "not allowed: registry.terraform.io/attacker/thing",
		},
		{
			name: "subdirectories_without_modules_json_are_ignored",
			files: map[string]string{
				"main.tf":               `resource "google_project" "p" {}`,
				"unused/other/main.tf":  `resource "aws_instance" "i" {}`,
				"README.md":             "not terraform",
				"terraform.tfvars.json": `{"a": "b"}`,
			},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{hashicorp("google")},
		},
		{
			name: "ignored_files_skipped",
			files: map[string]string{
				"main.tf":         `resource "google_project" "p" {}`,
				".hidden.tf":      "this is { not valid hcl",
				"main.tf~":        "this is { not valid hcl",
				"#main.tf#":       "this is { not valid hcl",
				".hidden.tf.json": "{",
			},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []*ProviderSource{hashicorp("google")},
		},
		{
			name:     "unparseable_hcl_rejected",
			files:    map[string]string{"main.tf": `resource "google_project" "p" {}`, "broken.tf": "this is { not valid hcl"},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to parse",
		},
		{
			name:     "unparseable_json_rejected",
			files:    map[string]string{"main.tf.json": "{"},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to parse",
		},
		{
			name:     "unparseable_hcl_rejected_when_matcher_disabled",
			files:    map[string]string{"broken.tf": "this is { not valid hcl"},
			patterns: nil,
			expErr:   "failed to parse",
		},
		{
			name:     "resource_missing_label_rejected",
			files:    map[string]string{"main.tf": `resource "attacker_thing" {}`},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to analyze",
		},
		{
			name: "provider_block_extra_label_rejected",
			files: map[string]string{"main.tf": `
provider "google" "extra" {}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to analyze",
		},
		{
			name: "required_providers_block_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil {
      source = "attacker/evil"
    }
  }
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to analyze",
		},
		{
			name: "non_literal_source_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = { source = var.source }
  }
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   `invalid required_providers entry "google"`,
		},
		{
			name: "non_string_source_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = { source = 5 }
  }
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "source must be a literal string",
		},
		{
			name: "duplicate_source_key_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      source = "attacker/evil"
    }
  }
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "source is set more than once",
		},
		{
			name: "non_object_non_string_entry_rejected",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    google = 5
  }
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "must be an object or a version constraint string",
		},
		{
			name: "invalid_provider_meta_argument_rejected",
			files: map[string]string{"main.tf": `
resource "google_project" "p" {
  provider = lookup(local.providers, "evil")
}
`},
			patterns: []string{"hashicorp/google"},
			expErr:   "failed to analyze",
		},
		{
			name:        "no_patterns_rejects_declared",
			files:       map[string]string{"main.tf": `resource "google_project" "p" {}`},
			patterns:    nil,
			expDeclared: []*ProviderSource{hashicorp("google")},
			expErr:      `no provider sources are allowed; set -allowed-provider-sources, or use "*" to allow all`,
		},
		{
			name:        "no_patterns_allows_builtin_only",
			files:       map[string]string{"main.tf": `resource "terraform_data" "d" {}`},
			patterns:    nil,
			expDeclared: []*ProviderSource{},
		},
		{
			name: "allow_all",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil = { source = "attacker/evil" }
  }
}
`},
			patterns: []string{"*"},
			expDeclared: []*ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "evil"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tc.files {
				mustWrite(t, filepath.Join(dir, name), content)
			}

			m, err := NewSourceMatcher(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}

			// Allow every provisioner so that only providers are checked.
			cfg, err := CheckConfiguration(context.Background(), dir, &Policy{
				ProviderSources:    m,
				ProvisionerSources: mustProvisionerMatcher(t, "*"),
			})
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			var got []*ProviderSource
			if cfg != nil {
				got = cfg.ProviderSources
			}
			if diff := cmp.Diff(tc.expDeclared, got); diff != "" {
				t.Errorf("declared providers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckConfiguration_Provisioners(t *testing.T) {
	t.Parallel()

	builtin := func(typ string) *ProvisionerSource {
		return &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: typ}
	}
	plugin := func(typ string) *ProvisionerSource {
		return &ProvisionerSource{Origin: ProvisionerOriginPlugin, Type: typ}
	}

	const localExec = `
resource "terraform_data" "a" {
  provisioner "local-exec" {
    command = "true"
  }
}
`

	const removedLocalExec = `
removed {
  from = terraform_data.a
  lifecycle {
    destroy = true
  }
  provisioner "local-exec" {
    when    = destroy
    command = "true"
  }
}
`

	cases := []struct {
		name        string
		files       map[string]string
		patterns    []string
		expDeclared []*ProvisionerSource
		expErr      string
	}{
		{
			name:        "no_provisioners",
			files:       map[string]string{"main.tf": `resource "terraform_data" "a" {}`},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{},
		},
		{
			name:        "denied_by_default",
			files:       map[string]string{"main.tf": localExec},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
			expErr:      `terraform uses provisioners that are not allowed: builtin/local-exec (no provisioners are allowed; set -allowed-provisioner-sources, or use "*" to allow all)`,
		},
		{
			name:        "builtin_allowed_by_bare_name",
			files:       map[string]string{"main.tf": localExec},
			patterns:    []string{"local-exec"},
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
		},
		{
			name: "plugin_rejected_by_builtin_wildcard",
			files: map[string]string{"main.tf": `
resource "terraform_data" "a" {
  provisioner "file" {
    content     = "x"
    destination = "/tmp/x"
  }
  provisioner "evil" {}
}
`},
			patterns:    []string{"builtin/*"},
			expDeclared: []*ProvisionerSource{builtin("file"), plugin("evil")},
			expErr:      "not allowed: plugin/evil (allowed provisioner sources: builtin/*)",
		},
		{
			name: "allow_all",
			files: map[string]string{"main.tf": `
resource "terraform_data" "a" {
  provisioner "evil" {}
}
`},
			patterns:    []string{"*"},
			expDeclared: []*ProvisionerSource{plugin("evil")},
		},
		{
			name: "json_config",
			files: map[string]string{"main.tf.json": `{
  "resource": {
    "terraform_data": {
      "a": {"provisioner": [{"remote-exec": {"inline": ["true"]}}]}
    }
  }
}`},
			patterns:    []string{"local-exec"},
			expDeclared: []*ProvisionerSource{builtin("remote-exec")},
			expErr:      "not allowed: builtin/remote-exec",
		},
		{
			// e.g. a provisioner in a remote module, which is only visible after
			// terraform init has installed it.
			name: "module_provisioner_rejected",
			files: map[string]string{
				"main.tf":                         `module "child" { source = "./modules/child" }`,
				"modules/child/main.tf":           localExec,
				".terraform/modules/modules.json": `{"Modules":[{"Key":"","Source":"","Dir":"."},{"Key":"child","Source":"./modules/child","Dir":"modules/child"}]}`,
			},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
			expErr:      "not allowed: builtin/local-exec",
		},
		{
			name: "non_module_subdirectories_ignored",
			files: map[string]string{
				"main.tf":           `resource "terraform_data" "a" {}`,
				"scratch/unused.tf": localExec,
			},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{},
		},
		{
			name: "provisioner_missing_type_rejected",
			files: map[string]string{"main.tf": `
resource "terraform_data" "a" {
  provisioner {}
}
`},
			patterns: []string{"*"},
			expErr:   "failed to analyze",
		},
		{
			name:     "unparseable_rejected",
			files:    map[string]string{"main.tf": "this is { not valid hcl"},
			patterns: []string{"*"},
			expErr:   "failed to parse",
		},
		{
			name:        "removed_block_destroy_provisioner_rejected",
			files:       map[string]string{"main.tf": removedLocalExec},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
			expErr:      "terraform uses provisioners that are not allowed: builtin/local-exec",
		},
		{
			name:        "removed_block_destroy_provisioner_allowed",
			files:       map[string]string{"main.tf": removedLocalExec},
			patterns:    []string{"local-exec"},
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
		},
		{
			name: "removed_block_json_rejected",
			files: map[string]string{"main.tf.json": `{
  "removed": [{
    "from": "terraform_data.a",
    "lifecycle": {"destroy": true},
    "provisioner": [{"local-exec": {"when": "destroy", "command": "true"}}]
  }]
}`},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
			expErr:      "not allowed: builtin/local-exec",
		},
		{
			name: "removed_block_json_allowed",
			files: map[string]string{"main.tf.json": `{
  "removed": {
    "from": "terraform_data.a",
    "provisioner": {"local-exec": {"when": "destroy", "command": "true"}}
  }
}`},
			patterns:    []string{"local-exec"},
			expDeclared: []*ProvisionerSource{builtin("local-exec")},
		},
		{
			name:        "removed_block_without_provisioner",
			files:       map[string]string{"main.tf": "removed {\n  from = terraform_data.a\n}\n"},
			patterns:    nil,
			expDeclared: []*ProvisionerSource{},
		},
		{
			name: "removed_block_provisioner_missing_type_rejected",
			files: map[string]string{"main.tf": `
removed {
  from = terraform_data.a
  provisioner {}
}
`},
			patterns: []string{"*"},
			expErr:   "failed to analyze",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tc.files {
				mustWrite(t, filepath.Join(dir, name), content)
			}

			m, err := NewProvisionerSourceMatcher(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}

			// Allow every provider so that only provisioners are checked.
			cfg, err := CheckConfiguration(context.Background(), dir, &Policy{
				ProviderSources:    mustProviderMatcher(t, "*"),
				ProvisionerSources: m,
			})
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			var got []*ProvisionerSource
			if cfg != nil {
				got = cfg.ProvisionerSources
			}
			if diff := cmp.Diff(tc.expDeclared, got); diff != "" {
				t.Errorf("declared provisioners mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckConfiguration_ReportsProvidersAndProvisioners(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.tf"), `
resource "google_project" "p" {
  provisioner "local-exec" {
    command = "true"
  }
}
`)

	_, err := CheckConfiguration(context.Background(), dir, &Policy{})
	for _, want := range []string{
		"terraform uses provider sources that are not allowed: registry.terraform.io/hashicorp/google",
		"terraform uses provisioners that are not allowed: builtin/local-exec",
	} {
		if diff := testutil.DiffErrString(err, want); diff != "" {
			t.Error(diff)
		}
	}
}

func TestCheckConfiguration_NilPolicyAllowsNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.tf"), `resource "google_project" "p" {}`)

	if _, err := CheckConfiguration(context.Background(), dir, nil); err == nil {
		t.Errorf("expected an error")
	}
}

func TestCheckBeforeInit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		files               map[string]string
		dirs                []string
		providerPatterns    []string
		provisionerPatterns []string
		expErr              string
		expRemoved          []string
	}{
		{
			name: "allowed",
			files: map[string]string{
				"main.tf":                `resource "google_project" "p" {}`,
				".terraform/plugin_path": `["/tmp"]`,
			},
			providerPatterns: []string{"hashicorp/google"},
			expRemoved:       []string{".terraform/plugin_path"},
		},
		{
			name: "local_plugin_rejected_before_configuration",
			files: map[string]string{
				"main.tf":                    "this is { not valid hcl",
				"terraform-provisioner-evil": "#!/bin/sh",
			},
			expErr: "found provisioner plugins",
		},
		{
			name:   "disallowed_provider",
			files:  map[string]string{"main.tf": `resource "google_project" "p" {}`},
			expErr: "terraform uses provider sources that are not allowed",
		},
		{
			name: "disallowed_provisioner",
			files: map[string]string{"main.tf": `
resource "terraform_data" "d" {
  provisioner "local-exec" {
    command = "true"
  }
}
`},
			expErr: "terraform uses provisioners that are not allowed: builtin/local-exec",
		},
		{
			name: "disallowed_removed_block_provisioner",
			files: map[string]string{"main.tf": `
removed {
  from = terraform_data.d
  provisioner "local-exec" {
    when    = destroy
    command = "true"
  }
}
`},
			expErr: "terraform uses provisioners that are not allowed: builtin/local-exec",
		},
		{
			name: "disallowed_removed_block_provisioner",
			files: map[string]string{"main.tf": `
removed {
  from = terraform_data.d
  provisioner "local-exec" {
    when    = destroy
    command = "true"
  }
}
`},
			expErr: "terraform uses provisioners that are not allowed: builtin/local-exec",
		},
		{
			name: "corrupt_modules_json_fails_closed",
			files: map[string]string{
				"main.tf":                         `resource "google_project" "p" {}`,
				".terraform/modules/modules.json": `{"Modules": [`,
			},
			providerPatterns: []string{"hashicorp/google"},
			expErr:           "failed to parse modules.json",
		},
		{
			name:             "unreadable_modules_json_fails_closed",
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			dirs:             []string{".terraform/modules/modules.json"},
			providerPatterns: []string{"hashicorp/google"},
			expErr:           "failed to read modules.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
			dir := t.TempDir()
			for name, content := range tc.files {
				mustWrite(t, filepath.Join(dir, name), content)
			}
			for _, d := range tc.dirs {
				mustMkdir(t, filepath.Join(dir, d))
			}

			err := CheckBeforeInit(ctx, dir, &Policy{
				ProviderSources:    mustProviderMatcher(t, tc.providerPatterns...),
				ProvisionerSources: mustProvisionerMatcher(t, tc.provisionerPatterns...),
			})
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			for _, p := range tc.expRemoved {
				if _, err := os.Lstat(filepath.Join(dir, p)); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected %q to be removed, got err=%v", p, err)
				}
			}
		})
	}
}

func TestCheckBeforeExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		files               map[string]string
		dirs                []string
		installed           []string
		providerPatterns    []string
		provisionerPatterns []string
		nilPolicy           bool
		expErr              string
	}{
		{
			name:             "allowed",
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:        []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			providerPatterns: []string{"hashicorp/google"},
		},
		{
			name:             "disallowed_installed_provider",
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed:        []string{"registry.terraform.io/hashicorp/external/2.0.0/linux_amd64"},
			providerPatterns: []string{"hashicorp/google"},
			expErr:           "not allowed: registry.terraform.io/hashicorp/external",
		},
		{
			// A provisioner in a module that terraform init installed.
			name: "disallowed_module_provisioner",
			files: map[string]string{
				"main.tf":                         `module "m" { source = "example.com/acme/m/x" }`,
				".terraform/modules/modules.json": `{"Modules":[{"Key":"","Source":"","Dir":"."},{"Key":"m","Source":"example.com/acme/m/x","Dir":".terraform/modules/m"}]}`,
				".terraform/modules/m/main.tf": `
resource "terraform_data" "d" {
  provisioner "remote-exec" {
    inline = ["true"]
  }
}
`,
			},
			provisionerPatterns: []string{"local-exec"},
			expErr:              "terraform uses provisioners that are not allowed: builtin/remote-exec",
		},
		{
			name:      "nil_policy_allows_nothing",
			files:     map[string]string{"main.tf": `resource "google_project" "p" {}`},
			installed: []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			nilPolicy: true,
			expErr:    "no provider sources are allowed",
		},
		{
			name: "corrupt_modules_json_fails_closed",
			files: map[string]string{
				"main.tf":                         `resource "google_project" "p" {}`,
				".terraform/modules/modules.json": `not json`,
			},
			installed:        []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			providerPatterns: []string{"hashicorp/google"},
			expErr:           "failed to parse modules.json",
		},
		{
			name:             "unreadable_modules_json_fails_closed",
			files:            map[string]string{"main.tf": `resource "google_project" "p" {}`},
			dirs:             []string{".terraform/modules/modules.json"},
			installed:        []string{"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64"},
			providerPatterns: []string{"hashicorp/google"},
			expErr:           "failed to read modules.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
			dir := t.TempDir()
			for name, content := range tc.files {
				mustWrite(t, filepath.Join(dir, name), content)
			}
			for _, d := range tc.dirs {
				mustMkdir(t, filepath.Join(dir, d))
			}
			for _, p := range tc.installed {
				mustMkdir(t, filepath.Join(dir, ".terraform", "providers", p))
			}

			var policy *Policy
			if !tc.nilPolicy {
				policy = &Policy{
					ProviderSources:    mustProviderMatcher(t, tc.providerPatterns...),
					ProvisionerSources: mustProvisionerMatcher(t, tc.provisionerPatterns...),
				}
			}
			err := CheckBeforeExecute(ctx, dir, policy)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func mustProviderMatcher(tb testing.TB, patterns ...string) *SourceMatcher {
	tb.Helper()

	m, err := NewSourceMatcher(patterns)
	if err != nil {
		tb.Fatal(err)
	}
	return m
}

func mustProvisionerMatcher(tb testing.TB, patterns ...string) *ProvisionerSourceMatcher {
	tb.Helper()

	m, err := NewProvisionerSourceMatcher(patterns)
	if err != nil {
		tb.Fatal(err)
	}
	return m
}
