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

package flags

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/checkterraform"
	"github.com/abcxyz/pkg/cli"
)

func TestSourceFlags_ProviderSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        []string
		expPatterns []string
		expAllowed  map[string]bool
	}{
		{
			name:        "default_allows_none",
			args:        nil,
			expPatterns: nil,
			expAllowed: map[string]bool{
				"hashicorp/google":                      false,
				"hashicorp/null":                        false,
				"hashicorp/external":                    false,
				"integrations/github":                   false,
				"registry.example.com/hashicorp/google": false,
			},
		},
		{
			name:        "explicit_namespace_wildcard",
			args:        []string{"-allowed-provider-sources=hashicorp/*"},
			expPatterns: []string{"registry.terraform.io/hashicorp/*"},
			expAllowed: map[string]bool{
				"hashicorp/google":    true,
				"hashicorp/external":  true,
				"integrations/github": false,
			},
		},
		{
			name:        "value_replaces_default",
			args:        []string{"-allowed-provider-sources=integrations/github"},
			expPatterns: []string{"registry.terraform.io/integrations/github"},
			expAllowed: map[string]bool{
				"hashicorp/google":    false,
				"integrations/github": true,
			},
		},
		{
			name:        "empty_value_allows_none",
			args:        []string{"-allowed-provider-sources="},
			expPatterns: nil,
			expAllowed: map[string]bool{
				"hashicorp/google": false,
			},
		},
		{
			name:        "wildcard_allows_all",
			args:        []string{"-allowed-provider-sources=*"},
			expPatterns: []string{"*"},
			expAllowed: map[string]bool{
				"hashicorp/google":                     true,
				"registry.example.com/acme/thing":      true,
				"registry.terraform.io/integrations/x": true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var p SourceFlags
			set := cli.NewFlagSet()
			p.RegisterSourceFlags(set.NewSection("OPTIONS"))
			if err := set.Parse(tc.args); err != nil {
				t.Fatal(err)
			}

			m, err := p.ProviderSourceMatcher()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expPatterns, m.Patterns()); diff != "" {
				t.Errorf("patterns mismatch (-want +got):\n%s", diff)
			}
			for raw, want := range tc.expAllowed {
				src, err := checkterraform.ParseProviderSource(raw)
				if err != nil {
					t.Fatal(err)
				}
				if got := m.Allowed(src); got != want {
					t.Errorf("Allowed(%q) = %t, want %t", raw, got, want)
				}
			}
		})
	}
}

func TestSourceFlags_ProvisionerSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        []string
		expPatterns []string
		expAllowed  map[string]bool
	}{
		{
			name:        "default_allows_none",
			args:        nil,
			expPatterns: nil,
			expAllowed: map[string]bool{
				"local-exec": false,
				"custom":     false,
			},
		},
		{
			name:        "builtin",
			args:        []string{"-allowed-provisioner-sources=local-exec"},
			expPatterns: []string{"builtin/local-exec"},
			expAllowed: map[string]bool{
				"local-exec":  true,
				"remote-exec": false,
			},
		},
		{
			name:        "wildcard_allows_all",
			args:        []string{"-allowed-provisioner-sources=*"},
			expPatterns: []string{"*"},
			expAllowed: map[string]bool{
				"local-exec": true,
				"custom":     true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var p SourceFlags
			set := cli.NewFlagSet()
			p.RegisterSourceFlags(set.NewSection("OPTIONS"))
			if err := set.Parse(tc.args); err != nil {
				t.Fatal(err)
			}

			m, err := p.ProvisionerSourceMatcher()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expPatterns, m.Patterns()); diff != "" {
				t.Errorf("patterns mismatch (-want +got):\n%s", diff)
			}
			for typ, want := range tc.expAllowed {
				if got := m.Allowed(checkterraform.ProvisionerSourceForType(typ)); got != want {
					t.Errorf("Allowed(%q) = %t, want %t", typ, got, want)
				}
			}
		})
	}
}
