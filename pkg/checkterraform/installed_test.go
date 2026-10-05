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

func TestRemoveLocalProviderOverrides(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		dirs       []string
		files      []string
		expErr     string
		expRemoved []string // must not exist afterwards
		expKept    []string // must still exist afterwards
	}{
		{
			name: "clean",
		},
		{
			name:   "terraform_d_rejected",
			dirs:   []string{"terraform.d/plugins/registry.terraform.io/hashicorp/google/9.9.9/linux_amd64"},
			expErr: "found terraform.d in",
		},
		{
			name:   "file_named_terraform_d_rejected",
			files:  []string{"terraform.d"},
			expErr: "found terraform.d in",
		},
		{
			name: "provider_caches_removed_rest_kept",
			dirs: []string{
				".terraform/providers/registry.terraform.io/hashicorp/google/9.9.9/linux_amd64",
				".terraform/plugins/registry.terraform.io/hashicorp/google/9.9.9/linux_amd64",
				".terraform/modules/child",
			},
			files: []string{
				".terraform/environment",
				".terraform/terraform.tfstate",
				".terraform/modules/modules.json",
			},
			expRemoved: []string{".terraform/providers", ".terraform/plugins"},
			expKept: []string{
				".terraform",
				".terraform/environment",
				".terraform/terraform.tfstate",
				".terraform/modules/modules.json",
			},
		},
		{
			name:    "dot_terraform_without_provider_cache",
			dirs:    []string{".terraform/modules"},
			expKept: []string{".terraform/modules"},
		},
		{
			name:   "dot_terraform_file_rejected",
			files:  []string{".terraform"},
			expErr: "is not a directory",
		},
		{
			name:    "unrelated_dirs",
			dirs:    []string{"modules/terraform.d", "nested/.terraform/providers"},
			files:   []string{".terraform.lock.hcl", "main.tf"},
			expKept: []string{"modules/terraform.d", "nested/.terraform/providers", ".terraform.lock.hcl"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

			dir := t.TempDir()
			for _, d := range tc.dirs {
				mustMkdir(t, filepath.Join(dir, d))
			}
			for _, f := range tc.files {
				mustWrite(t, filepath.Join(dir, f), "")
			}

			err := RemoveLocalProviderOverrides(ctx, dir)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}

			for _, p := range tc.expRemoved {
				if _, err := os.Lstat(filepath.Join(dir, p)); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected %q to be removed, got err=%v", p, err)
				}
			}
			for _, p := range tc.expKept {
				if _, err := os.Lstat(filepath.Join(dir, p)); err != nil {
					t.Errorf("expected %q to be kept: %v", p, err)
				}
			}
		})
	}
}

func TestRemoveLocalProviderOverrides_Symlinks(t *testing.T) {
	t.Parallel()

	t.Run("symlinked_dot_terraform_rejected", func(t *testing.T) {
		t.Parallel()

		ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
		dir := t.TempDir()
		outside := t.TempDir()
		mustMkdir(t, filepath.Join(outside, "providers"))
		if err := os.Symlink(outside, filepath.Join(dir, ".terraform")); err != nil {
			t.Fatal(err)
		}

		err := RemoveLocalProviderOverrides(ctx, dir)
		if diff := testutil.DiffErrString(err, "is not a directory"); diff != "" {
			t.Error(diff)
		}
		if _, err := os.Stat(filepath.Join(outside, "providers")); err != nil {
			t.Errorf("expected directory outside of dir to be untouched: %v", err)
		}
	})

	t.Run("symlinked_provider_cache_only_unlinked", func(t *testing.T) {
		t.Parallel()

		ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
		dir := t.TempDir()
		outside := t.TempDir()
		mustWrite(t, filepath.Join(outside, "keep"), "")
		mustMkdir(t, filepath.Join(dir, ".terraform"))
		if err := os.Symlink(outside, filepath.Join(dir, ".terraform", "providers")); err != nil {
			t.Fatal(err)
		}

		if err := RemoveLocalProviderOverrides(ctx, dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".terraform", "providers")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected symlink to be removed, got err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "keep")); err != nil {
			t.Errorf("expected symlink target to be untouched: %v", err)
		}
	})
}

func TestCheckInstalledProviderSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		patterns     []string
		installed    []string // relative to .terraform/providers
		strayFiles   []string // relative to .terraform/providers
		noProviders  bool
		expInstalled []ProviderSource
		expErr       string
	}{
		{
			name:        "no_providers_dir",
			patterns:    []string{"hashicorp/*"},
			noProviders: true,
		},
		{
			name:     "all_allowed",
			patterns: []string{"hashicorp/google", "hashicorp/google-beta"},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				"registry.terraform.io/hashicorp/google-beta/6.0.0/linux_amd64",
			},
			expInstalled: []ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google-beta"},
			},
		},
		{
			name:     "disallowed_namespace",
			patterns: []string{"hashicorp/*"},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				"registry.terraform.io/attacker/evil/1.0.0/linux_amd64",
			},
			expInstalled: []ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "evil"},
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
			},
			expErr: "not allowed: registry.terraform.io/attacker/evil (allowed provider sources: registry.terraform.io/hashicorp/*)",
		},
		{
			name:     "disallowed_hashicorp_external",
			patterns: []string{"hashicorp/google"},
			installed: []string{
				"registry.terraform.io/hashicorp/external/2.0.0/linux_amd64",
			},
			expInstalled: []ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "external"},
			},
			expErr: "not allowed: registry.terraform.io/hashicorp/external",
		},
		{
			name:     "disallowed_host",
			patterns: []string{"hashicorp/*"},
			installed: []string{
				"evil.example.com/hashicorp/google/6.0.0/linux_amd64",
			},
			expInstalled: []ProviderSource{
				{Hostname: "evil.example.com", Namespace: "hashicorp", Type: "google"},
			},
			expErr: "not allowed: evil.example.com/hashicorp/google",
		},
		{
			name:     "matcher_disabled_returns_installed",
			patterns: nil,
			installed: []string{
				"registry.terraform.io/attacker/evil/1.0.0/linux_amd64",
			},
			expInstalled: []ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "attacker", Type: "evil"},
			},
		},
		{
			name:     "ignores_stray_files",
			patterns: []string{"hashicorp/google"},
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
			},
			strayFiles: []string{
				"README",
				"registry.terraform.io/README",
				"registry.terraform.io/hashicorp/README",
			},
			expInstalled: []ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			providersDir := filepath.Join(dir, ".terraform", "providers")
			if !tc.noProviders {
				mustMkdir(t, providersDir)
			}
			for _, p := range tc.installed {
				mustMkdir(t, filepath.Join(providersDir, p))
			}
			for _, f := range tc.strayFiles {
				mustWrite(t, filepath.Join(providersDir, f), "")
			}

			m, err := NewSourceMatcher(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}

			got, err := CheckInstalledProviderSources(context.Background(), dir, m)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
			if diff := cmp.Diff(tc.expInstalled, got); diff != "" {
				t.Errorf("installed providers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckInstalledProviderSources_Symlinks(t *testing.T) {
	t.Parallel()

	// Terraform's plugin cache creates symlinks into .terraform/providers. Make
	// sure symlinked directories are still discovered and checked.
	dir := t.TempDir()
	cache := t.TempDir()
	mustMkdir(t, filepath.Join(cache, "evil", "1.0.0", "linux_amd64"))

	nsDir := filepath.Join(dir, ".terraform", "providers", "registry.terraform.io", "attacker")
	mustMkdir(t, nsDir)
	if err := os.Symlink(filepath.Join(cache, "evil"), filepath.Join(nsDir, "evil")); err != nil {
		t.Fatal(err)
	}

	m, err := NewSourceMatcher([]string{"hashicorp/*"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CheckInstalledProviderSources(context.Background(), dir, m)
	if diff := testutil.DiffErrString(err, "not allowed: registry.terraform.io/attacker/evil"); diff != "" {
		t.Error(diff)
	}
}

func mustMkdir(tb testing.TB, p string) {
	tb.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		tb.Fatal(err)
	}
}

func mustWrite(tb testing.TB, p, content string) {
	tb.Helper()
	mustMkdir(tb, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		tb.Fatal(err)
	}
}
