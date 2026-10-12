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

func TestProvisionerSourceForType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typ string
		exp *ProvisionerSource
	}{
		{typ: "local-exec", exp: &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: "local-exec"}},
		{typ: "remote-exec", exp: &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: "remote-exec"}},
		{typ: "file", exp: &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: "file"}},
		{typ: "evil", exp: &ProvisionerSource{Origin: ProvisionerOriginPlugin, Type: "evil"}},
		{typ: "Local-Exec", exp: &ProvisionerSource{Origin: ProvisionerOriginPlugin, Type: "Local-Exec"}},
	}

	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tc.exp, ProvisionerSourceForType(tc.typ)); diff != "" {
				t.Errorf("ProvisionerSourceForType(%q) mismatch (-want +got):\n%s", tc.typ, diff)
			}
		})
	}
}

func TestParseProvisionerSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		in     string
		exp    *ProvisionerSource
		expErr string
	}{
		{
			name: "bare_type_implies_builtin",
			in:   "local-exec",
			exp:  &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: "local-exec"},
		},
		{
			name: "builtin",
			in:   " builtin/file ",
			exp:  &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: "file"},
		},
		{
			name: "plugin",
			in:   "plugin/custom",
			exp:  &ProvisionerSource{Origin: ProvisionerOriginPlugin, Type: "custom"},
		},
		{
			name:   "bare_non_builtin",
			in:     "custom",
			expErr: `"custom" is not a Terraform built-in provisioner (file, local-exec, remote-exec); use plugin/custom for a plugin`,
		},
		{
			name:   "unknown_origin",
			in:     "registry/custom",
			expErr: `origin must be "builtin" or "plugin"`,
		},
		{
			name:   "empty",
			in:     "",
			expErr: "must not be empty",
		},
		{
			name:   "empty_segment",
			in:     "plugin/",
			expErr: "segments must be non-empty",
		},
		{
			name:   "too_many_segments",
			in:     "a/b/c",
			expErr: "expected [ORIGIN/]TYPE",
		},
		{
			name:   "wildcard",
			in:     "builtin/*",
			expErr: "wildcards are not permitted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseProvisionerSource(tc.in)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(tc.exp, got); diff != "" {
				t.Errorf("ParseProvisionerSource(%q) mismatch (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func TestNewProvisionerSourceMatcher(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		patterns    []string
		expPatterns []string
		expErr      string
	}{
		{
			name:     "nil",
			patterns: nil,
		},
		{
			name:     "blank_only",
			patterns: []string{"", "  "},
		},
		{
			name:        "normalizes_and_dedupes",
			patterns:    []string{"local-exec", "builtin/local-exec", "plugin/custom", "builtin/*", "*/file"},
			expPatterns: []string{"builtin/local-exec", "plugin/custom", "builtin/*", "*/file"},
		},
		{
			name:        "allow_all",
			patterns:    []string{" * ", "*", "local-exec"},
			expPatterns: []string{"*", "builtin/local-exec"},
		},
		{
			name:     "rejects_exclusion",
			patterns: []string{"*", "!builtin/local-exec"},
			expErr:   "exclusions are not supported",
		},
		{
			name:     "rejects_literal_non_builtin",
			patterns: []string{"builtin/custom"},
			expErr:   `"custom" is not a Terraform built-in provisioner`,
		},
		{
			name:     "rejects_bare_non_builtin",
			patterns: []string{"custom"},
			expErr:   `"custom" is not a Terraform built-in provisioner`,
		},
		{
			name:     "rejects_unknown_origin",
			patterns: []string{"registry/custom"},
			expErr:   `origin must be "builtin", "plugin" or "*"`,
		},
		{
			name:     "rejects_bad_glob",
			patterns: []string{"plugin/[custom"},
			expErr:   "syntax error in pattern",
		},
		{
			name:     "rejects_too_many_segments",
			patterns: []string{"a/b/c"},
			expErr:   "expected [ORIGIN/]TYPE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewProvisionerSourceMatcher(tc.patterns)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.expPatterns, m.Patterns()); diff != "" {
				t.Errorf("Patterns() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProvisionerSourceMatcher_Allowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		patterns []string
		typ      string
		exp      bool
	}{
		{
			name:     "no_patterns_denies_builtin",
			patterns: nil,
			typ:      "local-exec",
			exp:      false,
		},
		{
			name:     "blank_patterns_deny_everything",
			patterns: []string{"", " "},
			typ:      "file",
			exp:      false,
		},
		{
			name:     "bare_type_allows_builtin",
			patterns: []string{"local-exec"},
			typ:      "local-exec",
			exp:      true,
		},
		{
			name:     "builtin_does_not_allow_other_builtin",
			patterns: []string{"builtin/local-exec"},
			typ:      "remote-exec",
			exp:      false,
		},
		{
			name:     "builtin_wildcard_allows_all_builtins",
			patterns: []string{"builtin/*"},
			typ:      "remote-exec",
			exp:      true,
		},
		{
			name:     "builtin_wildcard_does_not_allow_plugins",
			patterns: []string{"builtin/*"},
			typ:      "custom",
			exp:      false,
		},
		{
			name:     "plugin_exact",
			patterns: []string{"plugin/custom"},
			typ:      "custom",
			exp:      true,
		},
		{
			name:     "plugin_does_not_allow_builtin_of_same_name",
			patterns: []string{"plugin/local-exec"},
			typ:      "local-exec",
			exp:      false,
		},
		{
			name:     "origin_wildcard",
			patterns: []string{"*/custom"},
			typ:      "custom",
			exp:      true,
		},
		{
			name:     "allow_all_allows_builtin",
			patterns: []string{"*"},
			typ:      "local-exec",
			exp:      true,
		},
		{
			name:     "allow_all_allows_plugin",
			patterns: []string{"*"},
			typ:      "custom",
			exp:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewProvisionerSourceMatcher(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}
			src := ProvisionerSourceForType(tc.typ)
			if got := m.Allowed(src); got != tc.exp {
				t.Errorf("Allowed(%q) with patterns %q = %t, want %t", src, tc.patterns, got, tc.exp)
			}
		})
	}
}

func TestProvisionerSourceMatcher_Allowed_Nil(t *testing.T) {
	t.Parallel()

	m, err := NewProvisionerSourceMatcher([]string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Allowed(nil) {
		t.Errorf("Allowed(nil) = true, want false")
	}

	var nilMatcher *ProvisionerSourceMatcher
	if nilMatcher.Allowed(ProvisionerSourceForType("local-exec")) {
		t.Errorf("nil matcher Allowed() = true, want false")
	}
}
