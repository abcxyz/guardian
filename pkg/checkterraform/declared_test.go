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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/testutil"
)

func TestCheckDeclaredProviderSources(t *testing.T) {
	t.Parallel()

	hashicorp := func(typ string) ProviderSource {
		return ProviderSource{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: typ}
	}

	cases := []struct {
		name        string
		files       map[string]string
		patterns    []string
		expDeclared []ProviderSource
		expErr      string
	}{
		{
			name:        "empty",
			files:       map[string]string{"main.tf": ""},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []ProviderSource{},
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
			expDeclared: []ProviderSource{hashicorp("google"), hashicorp("google-beta")},
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
			expDeclared: []ProviderSource{
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
			expDeclared: []ProviderSource{hashicorp("external")},
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
			expDeclared: []ProviderSource{hashicorp("external")},
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
			expDeclared: []ProviderSource{hashicorp("google-beta")},
		},
		{
			name: "builtin_terraform_provider_skipped",
			files: map[string]string{"main.tf": `
data "terraform_remote_state" "s" {}
resource "terraform_data" "d" {}
`},
			patterns:    []string{"hashicorp/google"},
			expDeclared: []ProviderSource{},
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
			expDeclared: []ProviderSource{hashicorp("google")},
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
			expDeclared: []ProviderSource{
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
			expDeclared: []ProviderSource{
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
			expDeclared: []ProviderSource{hashicorp("google")},
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
			expDeclared: []ProviderSource{hashicorp("google")},
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
			name: "matcher_disabled",
			files: map[string]string{"main.tf": `
terraform {
  required_providers {
    evil = { source = "attacker/evil" }
  }
}
`},
			patterns: nil,
			expDeclared: []ProviderSource{
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

			got, err := CheckDeclaredProviderSources(context.Background(), dir, m)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			if diff := cmp.Diff(tc.expDeclared, got); diff != "" {
				t.Errorf("declared providers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
