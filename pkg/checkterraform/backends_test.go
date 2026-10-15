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

// testDefaultBackendAttributes mirrors flags.DefaultAllowedBackendAttributes,
// which cannot be imported here without an import cycle.
var testDefaultBackendAttributes = []string{
	"gcs/bucket", "gcs/prefix", "gcs/kms_encryption_key",
	"s3/bucket", "s3/key", "s3/region", "s3/dynamodb_table", "s3/encrypt", "s3/use_lockfile", "s3/kms_key_id",
	"azurerm/resource_group_name", "azurerm/storage_account_name", "azurerm/container_name",
	"azurerm/key", "azurerm/use_azuread_auth", "azurerm/use_oidc",
}

func TestCheckConfiguration_Backends(t *testing.T) {
	t.Parallel()

	tf := func(body string) map[string]string {
		return map[string]string{"main.tf": "terraform {\n" + body + "\n}\n"}
	}
	tfJSON := func(body string) map[string]string {
		return map[string]string{"main.tf.json": `{"terraform": ` + body + `}`}
	}

	cases := []struct {
		name    string
		files   map[string]string
		allowed []string // nil means testDefaultBackendAttributes
		expErr  string
	}{
		// No backend block is always fine. A backend type with no entries is
		// rejected even when its block is empty, since an empty block still
		// initializes that backend from the runner's ambient configuration.
		{
			name:    "no_backend",
			files:   tf(""),
			allowed: []string{},
		},
		{
			name:    "bare_backend_type_not_allowed",
			files:   tf(`backend "kubernetes" {}`),
			allowed: []string{},
			expErr:  `backend type "kubernetes" is not allowed`,
		},
		{
			name:    "bare_cloud_not_allowed",
			files:   tf("cloud {}"),
			allowed: []string{},
			expErr:  `backend type "cloud" is not allowed`,
		},
		{
			name:    "bare_backend_json_not_allowed",
			files:   tfJSON(`{"backend": {"local": {}}}`),
			allowed: []string{},
			expErr:  `backend type "local" is not allowed`,
		},
		{
			name:   "bare_local_not_in_default",
			files:  tf(`backend "local" {}`),
			expErr: `backend type "local" is not allowed`,
		},
		{
			name:  "bare_gcs_allowed_by_default",
			files: tf(`backend "gcs" {}`),
		},
		{
			name:    "bare_local_allowed_with_entry",
			files:   tf(`backend "local" {}`),
			allowed: []string{"local/path"},
		},
		{
			name:    "bare_local_allowed_with_type_wildcard",
			files:   tf(`backend "local" {}`),
			allowed: []string{"local/*"},
		},

		// Realistic configurations pass with the default allowlist.
		{
			name: "gcs",
			files: tf(`
  backend "gcs" {
    bucket             = "my-bucket"
    prefix             = "prod/state"
    kms_encryption_key = "projects/p/locations/us/keyRings/r/cryptoKeys/k"
  }`),
		},
		// encryption_key is a customer-supplied key that also accepts a file
		// path, so it stays out of the default.
		{
			name:   "gcs_encryption_key_not_default",
			files:  tf(`backend "gcs" { encryption_key = "/etc/passwd" }`),
			expErr: `attribute "encryption_key" is not allowed`,
		},
		{
			name:  "gcs_json",
			files: tfJSON(`{"backend": {"gcs": {"//": "comment", "bucket": "my-bucket", "prefix": "prod/state"}}}`),
		},
		{
			name: "s3",
			files: tf(`
  backend "s3" {
    bucket         = "my-bucket"
    key            = "prod/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "locks"
    encrypt        = true
    use_lockfile   = true
    kms_key_id     = "arn:aws:kms:us-east-1:123456789012:key/abc"
  }`),
		},
		// sse_customer_key is a raw customer-supplied key, so it stays out of
		// the default.
		{
			name:   "s3_sse_customer_key_not_default",
			files:  tf(`backend "s3" { sse_customer_key = "x" }`),
			expErr: `attribute "sse_customer_key" is not allowed`,
		},
		{
			name: "azurerm",
			files: tf(`
  backend "azurerm" {
    resource_group_name  = "rg"
    storage_account_name = "sa"
    container_name       = "tfstate"
    key                  = "prod.terraform.tfstate"
    use_oidc             = true
    use_azuread_auth     = true
  }`),
		},

		// Attributes are scoped to their backend type: an attribute allowed for
		// one type is not allowed for another, and types outside the default
		// need their own entries.
		{
			name:   "attribute_scoped_to_type",
			files:  tf(`backend "oss" { bucket = "b" }`),
			expErr: `backend type "oss" is not allowed`,
		},
		{
			name:   "attribute_scoped_to_type_other_type_has_it",
			files:  tf(`backend "gcs" { key = "k" }`),
			expErr: `attribute "key" is not allowed`,
		},
		{
			name: "oci_with_entries",
			files: tf(`
  backend "oci" {
    bucket    = "my-bucket"
    namespace = "ns"
    key       = "prod/terraform.tfstate"
    region    = "us-phoenix-1"
  }`),
			allowed: []string{"oci/bucket", "oci/namespace", "oci/key", "oci/region"},
		},
		{
			name: "cloud_with_entries",
			files: tf(`
  cloud {
    organization = "foo"
    workspaces {
      tags    = ["app"]
      project = "p"
    }
  }`),
			allowed: []string{"cloud/organization", "cloud/workspaces.tags", "cloud/workspaces.project"},
		},
		{
			name:    "cloud_json_with_entries",
			files:   tfJSON(`{"cloud": {"organization": "foo", "workspaces": {"name": "prod"}}}`),
			allowed: []string{"cloud/organization", "cloud/workspaces.name"},
		},
		{
			name:    "cloud_json_block_list_with_entries",
			files:   tfJSON(`{"cloud": [{"organization": "foo", "workspaces": [{"name": "prod"}]}]}`),
			allowed: []string{"cloud/organization", "cloud/workspaces.name"},
		},

		// Credential, endpoint, identity and file path attributes are rejected.
		{
			name: "gcs_storage_custom_endpoint",
			files: tf(`
  backend "gcs" {
    bucket                  = "my-bucket"
    storage_custom_endpoint = "https://attacker.example.com/storage/v1/"
  }`),
			expErr: `attribute "storage_custom_endpoint" is not allowed`,
		},
		{
			name:   "gcs_storage_custom_endpoint_json",
			files:  tfJSON(`{"backend": {"gcs": {"bucket": "b", "storage_custom_endpoint": "https://attacker.example.com/"}}}`),
			expErr: `attribute "storage_custom_endpoint" is not allowed`,
		},
		{
			name: "s3_shared_config_files",
			files: tf(`
  backend "s3" {
    bucket              = "b"
    shared_config_files = ["./aws_config"]
  }`),
			expErr: `attribute "shared_config_files" is not allowed`,
		},
		{
			name: "azurerm_oidc_request_url",
			files: tf(`
  backend "azurerm" {
    oidc_request_url = "https://attacker.example.com"
  }`),
			expErr: `attribute "oidc_request_url" is not allowed`,
		},
		{
			name: "kubernetes_config_path",
			files: tf(`
  backend "kubernetes" {
    secret_suffix = "state"
    config_path   = "./kubeconfig"
  }`),
			expErr: `backend type "kubernetes" is not allowed`,
		},
		{
			name: "kubernetes_config_path_with_entry",
			files: tf(`
  backend "kubernetes" {
    secret_suffix = "state"
    config_path   = "./kubeconfig"
  }`),
			allowed: []string{"kubernetes/secret_suffix"},
			expErr:  `attribute "config_path" is not allowed`,
		},
		{
			name: "cloud_hostname",
			files: tf(`
  cloud {
    hostname     = "attacker.example.com"
    organization = "foo"
  }`),
			allowed: []string{"cloud/organization"},
			expErr:  `attribute "hostname" is not allowed`,
		},
		{
			name:   "local_path",
			files:  tf(`backend "local" { path = "./evil.tfstate" }`),
			expErr: `backend type "local" is not allowed`,
		},
		{
			name:   "local_path_json",
			files:  tfJSON(`{"backend": {"local": {"path": "./evil.tfstate"}}}`),
			expErr: `backend type "local" is not allowed`,
		},
		{
			name: "s3_nested_endpoints",
			files: tf(`
  backend "s3" {
    bucket = "b"
    endpoints {
      s3 = "https://attacker.example.com"
    }
  }`),
			expErr: `block "endpoints" is not allowed`,
		},
		{
			name: "s3_nested_assume_role",
			files: tf(`
  backend "s3" {
    bucket = "b"
    assume_role {
      role_arn = "arn:aws:iam::123456789012:role/x"
    }
  }`),
			expErr: `block "assume_role" is not allowed`,
		},
		{
			name:   "s3_nested_assume_role_json",
			files:  tfJSON(`{"backend": {"s3": {"assume_role": {"role_arn": "x"}}}}`),
			expErr: `attribute "assume_role" is not allowed`,
		},
		{
			name:    "cloud_nested_disallowed",
			files:   tfJSON(`{"cloud": {"workspaces": {"name": "x", "evil": "y"}}}`),
			allowed: []string{"cloud/workspaces.name"},
			expErr:  `attribute "workspaces.evil" is not allowed`,
		},

		// Custom allowlists.
		{
			name: "custom_allows_nested_attribute",
			files: tf(`
  backend "s3" {
    bucket = "b"
    assume_role {
      role_arn = "arn:aws:iam::123456789012:role/x"
    }
  }`),
			allowed: []string{"s3/bucket", "s3/assume_role.role_arn"},
		},
		{
			name:    "custom_allows_nested_attribute_json",
			files:   tfJSON(`{"backend": {"s3": {"bucket": "b", "assume_role": {"role_arn": "x"}}}}`),
			allowed: []string{"s3/bucket", "s3/assume_role.role_arn"},
		},
		{
			name: "custom_rejects_other_nested_attribute",
			files: tf(`
  backend "s3" {
    assume_role {
      role_arn     = "x"
      external_id  = "y"
    }
  }`),
			allowed: []string{"s3/assume_role.role_arn"},
			expErr:  `attribute "assume_role.external_id" is not allowed`,
		},
		{
			name: "custom_allows_whole_block",
			files: tf(`
  backend "s3" {
    assume_role {
      role_arn    = "x"
      external_id = "y"
    }
  }`),
			allowed: []string{"s3/assume_role"},
		},
		{
			name:    "empty_allowlist_rejects_everything",
			files:   tf(`backend "gcs" { bucket = "b" }`),
			allowed: []string{},
			expErr:  "allowed backend attributes: none",
		},
		{
			name:    "error_explains_how_to_allow",
			files:   tf(`backend "gcs" { bucket = "b" }`),
			allowed: []string{},
			expErr:  "add TYPE/ATTRIBUTE (e.g. gcs/bucket) to -allowed-backend-attributes",
		},

		// TYPE/* and * accept anything in the block, including nested blocks,
		// non-literal values and otherwise dangerous attributes.
		{
			name: "type_wildcard_allows_dangerous_attribute",
			files: tf(`
  backend "gcs" {
    bucket                  = "b"
    storage_custom_endpoint = "https://attacker.example.com/"
  }`),
			allowed: []string{"gcs/*"},
		},
		{
			name: "type_wildcard_allows_nested_block_and_expression",
			files: tf(`
  backend "s3" {
    bucket = var.b
    endpoints {
      s3 = "https://x"
    }
  }`),
			allowed: []string{"s3/*"},
		},
		{
			name:    "type_wildcard_is_scoped",
			files:   tf(`backend "gcs" { storage_custom_endpoint = "https://x" }`),
			allowed: []string{"s3/*", "gcs/bucket"},
			expErr:  `attribute "storage_custom_endpoint" is not allowed`,
		},
		{
			name:    "type_wildcard_other_type_still_rejected",
			files:   tf(`backend "kubernetes" {}`),
			allowed: []string{"gcs/*"},
			expErr:  `backend type "kubernetes" is not allowed`,
		},
		{
			name:    "allow_all_allows_any_backend",
			files:   tf(`backend "kubernetes" { config_path = "./k" }`),
			allowed: []string{"*"},
		},
		{
			name:    "allow_all_allows_cloud",
			files:   tf(`cloud { hostname = "x" }`),
			allowed: []string{"*"},
		},

		// Override files and other root module files are checked too.
		{
			name: "override_file_adds_disallowed_attribute",
			files: map[string]string{
				"main.tf":     "terraform {\n  backend \"gcs\" { bucket = \"b\" }\n}\n",
				"override.tf": "terraform {\n  backend \"gcs\" { storage_custom_endpoint = \"https://attacker.example.com/\" }\n}\n",
			},
			expErr: `attribute "storage_custom_endpoint" is not allowed`,
		},
		{
			name: "child_module_ignored",
			files: map[string]string{
				"main.tf":                         "terraform {\n  backend \"gcs\" { bucket = \"b\" }\n}\n",
				"modules/child/main.tf":           "terraform {\n  backend \"local\" { path = \"./evil.tfstate\" }\n}\n",
				".terraform/modules/modules.json": `{"Modules":[{"Key":"","Source":"","Dir":"."},{"Key":"child","Source":"./modules/child","Dir":"modules/child"}]}`,
			},
		},

		// Values must be static literals.
		{
			name:   "variable_value",
			files:  tf(`backend "gcs" { bucket = var.bucket }`),
			expErr: "must be a static literal value",
		},
		{
			name:   "function_value",
			files:  tf(`backend "gcs" { bucket = file("./bucket") }`),
			expErr: "must be a static literal value",
		},
		{
			name:   "local_value",
			files:  tf(`backend "gcs" { prefix = "${local.p}/x" }`),
			expErr: "must be a static literal value",
		},
		{
			name:   "template_value_json",
			files:  tfJSON(`{"backend": {"gcs": {"bucket": "${var.bucket}"}}}`),
			expErr: "without template sequences",
		},
		{
			name:   "template_directive_json",
			files:  tfJSON(`{"backend": {"gcs": {"bucket": "%{if true}b%{endif}"}}}`),
			expErr: "without template sequences",
		},
		{
			name:  "literal_template_allowed",
			files: tf(`backend "gcs" { bucket = "my-${"bucket"}" }`),
		},
		{
			name:    "list_literal_allowed",
			files:   tf("cloud {\n  workspaces { tags = [\"a\", \"b\"] }\n}"),
			allowed: []string{"cloud/workspaces.tags"},
		},

		// Fail closed on malformed blocks.
		{
			name:   "empty_backend_type",
			files:  tf(`backend "" {}`),
			expErr: "has no type",
		},
		{
			name:   "missing_backend_type",
			files:  tf(`backend {}`),
			expErr: "Missing type for backend",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tc.files {
				mustWrite(t, filepath.Join(dir, name), content)
			}

			allowed := tc.allowed
			if allowed == nil {
				allowed = testDefaultBackendAttributes
			}
			s, err := NewBackendAttributeSet(allowed)
			if err != nil {
				t.Fatal(err)
			}

			_, err = CheckConfiguration(context.Background(), dir, &Policy{BackendAttributes: s})
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewBackendAttributeSet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		entries    []string
		expEntries []string
		expErr     string
	}{
		{
			name:       "nil",
			entries:    nil,
			expEntries: nil,
		},
		{
			name:       "skips_empty_and_sorts",
			entries:    []string{"", " gcs/bucket ", "cloud/workspaces.name", "azurerm/key"},
			expEntries: []string{"azurerm/key", "cloud/workspaces.name", "gcs/bucket"},
		},
		{
			name:       "type_wildcard",
			entries:    []string{"s3/*", "gcs/bucket"},
			expEntries: []string{"gcs/bucket", "s3/*"},
		},
		{
			name:       "allow_all",
			entries:    []string{"*"},
			expEntries: []string{"*"},
		},
		{
			name:    "missing_type",
			entries: []string{"bucket"},
			expErr:  `invalid backend attribute "bucket": must be TYPE/ATTRIBUTE`,
		},
		{
			name:    "empty_type",
			entries: []string{"/bucket"},
			expErr:  `invalid backend attribute "/bucket"`,
		},
		{
			name:    "uppercase_type",
			entries: []string{"GCS/bucket"},
			expErr:  `invalid backend attribute "GCS/bucket"`,
		},
		{
			name:    "empty_path",
			entries: []string{"gcs/"},
			expErr:  `invalid backend attribute "gcs/"`,
		},
		{
			name:    "invalid_dot",
			entries: []string{"cloud/workspaces."},
			expErr:  `invalid backend attribute "cloud/workspaces."`,
		},
		{
			name:    "glob_in_path",
			entries: []string{"gcs/buck*"},
			expErr:  `invalid backend attribute "gcs/buck*"`,
		},
		{
			name:    "glob_type",
			entries: []string{"*/bucket"},
			expErr:  `invalid backend attribute "*/bucket"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := NewBackendAttributeSet(tc.entries)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.expEntries, s.Entries()); diff != "" {
				t.Errorf("entries mismatch (-want +got):\n%s", diff)
			}
		})
	}

	s, err := NewBackendAttributeSet([]string{"s3/assume_role", "cloud/workspaces.name", "oss/*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		typ, path string
		want      bool
	}{
		{
			typ:  "s3",
			path: "assume_role",
			want: true,
		},
		{
			typ:  "s3",
			path: "assume_role.role_arn",
			want: true,
		},
		{
			typ:  "s3",
			path: "bucket",
			want: false,
		},
		{
			typ:  "cloud",
			path: "workspaces",
			want: false,
		},
		{
			typ:  "cloud",
			path: "workspaces.name",
			want: true,
		},
		{
			typ:  "cloud",
			path: "workspaces.tags",
			want: false,
		},
		{
			typ:  "gcs",
			path: "assume_role",
			want: false,
		}, // scoped to s3
		{
			typ:  "oss",
			path: "anything.at.all",
			want: true,
		},
	} {
		if got := s.Allowed(tc.typ, tc.path); got != tc.want {
			t.Errorf("Allowed(%q, %q) = %t, want %t", tc.typ, tc.path, got, tc.want)
		}
	}
	for typ, want := range map[string]bool{"s3": true, "cloud": true, "oss": true, "gcs": false} {
		if got := s.AllowsType(typ); got != want {
			t.Errorf("AllowsType(%q) = %t, want %t", typ, got, want)
		}
	}
	var nilSet *BackendAttributeSet
	if nilSet.Allowed("gcs", "bucket") || nilSet.AllowsType("gcs") {
		t.Error("nil set allowed gcs")
	}
}

func TestCheckBeforeInit_RejectsBackendAttribute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.tf"), `
terraform {
  backend "gcs" {
    bucket                  = "my-bucket"
    storage_custom_endpoint = "https://attacker.example.com/storage/v1/"
  }
}
`)

	s, err := NewBackendAttributeSet(testDefaultBackendAttributes)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckBeforeInit(context.Background(), dir, &Policy{
		ProviderSources:    mustProviderMatcher(t, "hashicorp/google"),
		ProvisionerSources: mustProvisionerMatcher(t, "*"),
		BackendAttributes:  s,
	})
	if diff := testutil.DiffErrString(err, `attribute "storage_custom_endpoint" is not allowed`); diff != "" {
		t.Error(diff)
	}
}
