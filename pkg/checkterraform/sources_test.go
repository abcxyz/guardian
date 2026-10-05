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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/testutil"
)

func TestParseProviderSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		in     string
		exp    ProviderSource
		expErr string
	}{
		{
			name: "type_only",
			in:   "google",
			exp:  ProviderSource{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
		},
		{
			name: "namespace_and_type",
			in:   "integrations/github",
			exp:  ProviderSource{Hostname: "registry.terraform.io", Namespace: "integrations", Type: "github"},
		},
		{
			name: "fully_qualified",
			in:   "registry.example.com/acme/thing",
			exp:  ProviderSource{Hostname: "registry.example.com", Namespace: "acme", Type: "thing"},
		},
		{
			name: "mixed_case_and_whitespace",
			in:   "  Registry.Terraform.IO/HashiCorp/Google ",
			exp:  ProviderSource{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
		},
		{
			name: "builtin",
			in:   "terraform.io/builtin/terraform",
			exp:  ProviderSource{Hostname: "terraform.io", Namespace: "builtin", Type: "terraform"},
		},
		{
			name:   "empty",
			in:     "",
			expErr: "must not be empty",
		},
		{
			name:   "empty_segment",
			in:     "hashicorp//google",
			expErr: "segments must be non-empty",
		},
		{
			name:   "too_many_segments",
			in:     "a/b/c/d",
			expErr: "expected [HOSTNAME/]NAMESPACE/TYPE",
		},
		{
			name:   "wildcard",
			in:     "hashicorp/*",
			expErr: "wildcards are not permitted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseProviderSource(tc.in)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(tc.exp, got); diff != "" {
				t.Errorf("ParseProviderSource(%q) mismatch (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func TestProviderSource_IsBuiltin(t *testing.T) {
	t.Parallel()

	if got := (ProviderSource{Hostname: "terraform.io", Namespace: "builtin", Type: "terraform"}).IsBuiltin(); !got {
		t.Errorf("expected terraform.io/builtin/terraform to be builtin")
	}
	if got := (ProviderSource{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "terraform"}).IsBuiltin(); got {
		t.Errorf("expected registry.terraform.io/hashicorp/terraform to not be builtin")
	}
}

func TestNewSourceMatcher(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		patterns    []string
		expPatterns []string
		expEnabled  bool
		expErr      string
	}{
		{
			name:       "nil",
			patterns:   nil,
			expEnabled: false,
		},
		{
			name:       "blank_only",
			patterns:   []string{"", "  "},
			expEnabled: false,
		},
		{
			name:        "normalizes_and_dedupes",
			patterns:    []string{"hashicorp/*", "registry.terraform.io/hashicorp/*", "Integrations/GitHub"},
			expPatterns: []string{"registry.terraform.io/hashicorp/*", "registry.terraform.io/integrations/github"},
			expEnabled:  true,
		},
		{
			name:     "rejects_exclusion",
			patterns: []string{"hashicorp/*", "!hashicorp/external"},
			expErr:   "exclusions are not supported",
		},
		{
			name:     "rejects_bad_glob",
			patterns: []string{"hashicorp/[google"},
			expErr:   "syntax error in pattern",
		},
		{
			name:     "rejects_too_many_segments",
			patterns: []string{"a/b/c/d"},
			expErr:   "expected [HOSTNAME/]NAMESPACE/TYPE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewSourceMatcher(tc.patterns)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}
			if err != nil {
				return
			}
			if got := m.Enabled(); got != tc.expEnabled {
				t.Errorf("Enabled() = %t, want %t", got, tc.expEnabled)
			}
			if diff := cmp.Diff(tc.expPatterns, m.Patterns()); diff != "" {
				t.Errorf("Patterns() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSourceMatcher_Allowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		patterns []string
		source   string
		exp      bool
	}{
		{
			name:     "disabled_allows_everything",
			patterns: nil,
			source:   "attacker/evil",
			exp:      true,
		},
		{
			name:     "exact_match",
			patterns: []string{"hashicorp/google"},
			source:   "registry.terraform.io/hashicorp/google",
			exp:      true,
		},
		{
			name:     "exact_does_not_match_other_type",
			patterns: []string{"hashicorp/google"},
			source:   "hashicorp/external",
			exp:      false,
		},
		{
			name:     "namespace_wildcard_match",
			patterns: []string{"hashicorp/*"},
			source:   "hashicorp/google-beta",
			exp:      true,
		},
		{
			name:     "namespace_wildcard_rejects_other_namespace",
			patterns: []string{"hashicorp/*"},
			source:   "attacker/google",
			exp:      false,
		},
		{
			name:     "namespace_wildcard_rejects_other_host",
			patterns: []string{"hashicorp/*"},
			source:   "registry.example.com/hashicorp/google",
			exp:      false,
		},
		{
			name:     "type_only_pattern_implies_hashicorp",
			patterns: []string{"google"},
			source:   "hashicorp/google",
			exp:      true,
		},
		{
			name:     "builtin_always_allowed",
			patterns: []string{"hashicorp/google"},
			source:   "terraform.io/builtin/terraform",
			exp:      true,
		},
		{
			name:     "multiple_patterns",
			patterns: []string{"hashicorp/google", "integrations/github"},
			source:   "integrations/github",
			exp:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewSourceMatcher(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}
			src, err := ParseProviderSource(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Allowed(src); got != tc.exp {
				t.Errorf("Allowed(%q) with patterns %q = %t, want %t", src, tc.patterns, got, tc.exp)
			}
		})
	}
}
