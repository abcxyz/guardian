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

	"github.com/abcxyz/guardian/pkg/checkterraform/plugintest"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestRemoveLocalPluginOverrides(t *testing.T) {
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
			name:   "terraform_d_is_case_insensitive",
			dirs:   []string{"Terraform.D/plugins"},
			expErr: "found terraform.d in",
		},
		{
			name: "dot_terraform_removed_entirely",
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
			expRemoved: []string{".terraform"},
		},
		{
			name:       "dot_terraform_without_provider_cache_removed",
			dirs:       []string{".terraform/modules"},
			expRemoved: []string{".terraform"},
		},
		{
			name:       "dot_terraform_file_removed",
			files:      []string{".terraform"},
			expRemoved: []string{".terraform"},
		},
		{
			name:       "dot_terraform_is_case_insensitive",
			dirs:       []string{".Terraform/providers"},
			expRemoved: []string{".Terraform"},
		},
		{
			name:    "local_state_kept",
			files:   []string{"terraform.tfstate", "terraform.tfstate.backup", "main.tf"},
			expKept: []string{"terraform.tfstate", "terraform.tfstate.backup", "main.tf"},
		},
		{
			name:   "provisioner_plugin_dir_rejected",
			dirs:   []string{"terraform-provisioner-evil"},
			expErr: "found provisioner plugins in",
		},
		{
			name:   "provisioner_plugin_prefix_is_case_insensitive",
			files:  []string{"Terraform-Provisioner-evil"},
			expErr: "found provisioner plugins in",
		},
		{
			name:    "provisioner_plugin_in_subdir_ignored",
			files:   []string{"modules/child/terraform-provisioner-evil", "terraform-provisioner.md"},
			expKept: []string{"modules/child/terraform-provisioner-evil", "terraform-provisioner.md"},
		},
		{
			name:       "plugin_path_removed",
			files:      []string{".terraform/plugin_path", ".terraform/environment"},
			expRemoved: []string{".terraform"},
		},
		{
			name:       "plugin_path_dir_removed",
			dirs:       []string{".terraform/plugin_path/x"},
			expRemoved: []string{".terraform/plugin_path"},
		},
		{
			name:    "unrelated_dirs",
			dirs:    []string{"modules/terraform.d", "nested/.terraform/providers"},
			files:   []string{".terraform.lock.hcl", "main.tf", "terraform.tfvars", "terraform.tfstate.d/x/terraform.tfstate"},
			expKept: []string{"modules/terraform.d", "nested/.terraform/providers", ".terraform.lock.hcl", "terraform.tfvars", "terraform.tfstate.d/x/terraform.tfstate"},
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

			err := RemoveLocalPluginOverrides(ctx, dir)
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

func TestRemoveLocalPluginOverrides_Symlinks(t *testing.T) {
	t.Parallel()

	t.Run("symlinked_dot_terraform_only_unlinked", func(t *testing.T) {
		t.Parallel()

		ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
		dir := t.TempDir()
		outside := t.TempDir()
		mustMkdir(t, filepath.Join(outside, "providers"))
		if err := os.Symlink(outside, filepath.Join(dir, ".terraform")); err != nil {
			t.Fatal(err)
		}

		if err := RemoveLocalPluginOverrides(ctx, dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".terraform")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected symlink to be removed, got err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "providers")); err != nil {
			t.Errorf("expected directory outside of dir to be untouched: %v", err)
		}
	})

	// terraform init creates .terraform/modules with os.MkdirAll, which follows
	// symlinks, before it installs providers. A .terraform/modules symlink to
	// "." would let a module named "providers" be unpacked into
	// .terraform/providers, and one to ".." would let nested modules be unpacked
	// into terraform.d.
	for _, target := range []string{".", ".."} {
		t.Run("symlinked_modules_"+target, func(t *testing.T) {
			t.Parallel()

			ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
			dir := t.TempDir()
			mustWrite(t, filepath.Join(dir, "main.tf"), "")
			mustMkdir(t, filepath.Join(dir, ".terraform"))
			if err := os.Symlink(target, filepath.Join(dir, ".terraform", "modules")); err != nil {
				t.Fatal(err)
			}

			if err := RemoveLocalPluginOverrides(ctx, dir); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(dir, ".terraform")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("expected .terraform to be removed, got err=%v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "main.tf")); err != nil {
				t.Errorf("expected symlink target to be untouched: %v", err)
			}
		})
	}

	t.Run("symlinked_provisioner_plugin_rejected", func(t *testing.T) {
		t.Parallel()

		ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
		dir := t.TempDir()
		outside := t.TempDir()
		mustWrite(t, filepath.Join(outside, "plugin"), "")
		if err := os.Symlink(filepath.Join(outside, "plugin"), filepath.Join(dir, "terraform-provisioner-evil")); err != nil {
			t.Fatal(err)
		}

		err := RemoveLocalPluginOverrides(ctx, dir)
		if diff := testutil.DiffErrString(err, "found provisioner plugins in"); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("symlinked_plugin_path_only_unlinked", func(t *testing.T) {
		t.Parallel()

		ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
		dir := t.TempDir()
		outside := t.TempDir()
		mustWrite(t, filepath.Join(outside, "plugin_path"), "[]")
		mustMkdir(t, filepath.Join(dir, ".terraform"))
		if err := os.Symlink(filepath.Join(outside, "plugin_path"), filepath.Join(dir, ".terraform", "plugin_path")); err != nil {
			t.Fatal(err)
		}

		if err := RemoveLocalPluginOverrides(ctx, dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".terraform", "plugin_path")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected symlink to be removed, got err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "plugin_path")); err != nil {
			t.Errorf("expected symlink target to be untouched: %v", err)
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

		if err := RemoveLocalPluginOverrides(ctx, dir); err != nil {
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

func TestRemoveLocalPluginOverrides_ProvisionerPlugins(t *testing.T) {
	t.Parallel()

	for _, tc := range plugintest.Cases() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
			dir := t.TempDir()
			tc.Setup(t, dir, filepath.Join(t.TempDir(), "marker"))

			err := RemoveLocalPluginOverrides(ctx, dir)
			if tc.WantRejected {
				if diff := testutil.DiffErrString(err, plugintest.RejectedErr); diff != "" {
					t.Error(diff)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range tc.WantRemoved {
				if _, err := os.Lstat(filepath.Join(dir, p)); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected %q to be removed, got err=%v", p, err)
				}
			}
		})
	}
}

func TestVerifyPluginDirsAfterInit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		dirs     []string
		files    []string
		symlinks map[string]string // link -> target, relative to dir
		expErr   string
	}{
		{
			name: "clean",
		},
		{
			name: "fresh_init",
			dirs: []string{
				".terraform/providers/registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
				".terraform/modules/m",
			},
			files: []string{".terraform/terraform.tfstate", ".terraform/modules/modules.json", ".terraform.lock.hcl", "main.tf"},
		},
		{
			// terraform init links cached providers when TF_PLUGIN_CACHE_DIR is set.
			name:     "symlink_inside_providers_allowed",
			dirs:     []string{".terraform/providers/registry.terraform.io/hashicorp", "cache/google"},
			symlinks: map[string]string{".terraform/providers/registry.terraform.io/hashicorp/google": "../../../../cache/google"},
		},
		{
			name:   "terraform_d_rejected",
			dirs:   []string{"terraform.d/plugins"},
			expErr: "found terraform.d in",
		},
		{
			name:   "terraform_d_is_case_insensitive",
			files:  []string{"Terraform.d"},
			expErr: "found terraform.d in",
		},
		{
			name:   "provisioner_plugin_rejected",
			files:  []string{"terraform-provisioner-evil"},
			expErr: "found provisioner plugins in",
		},
		{
			name:   "provisioner_plugin_is_case_insensitive",
			dirs:   []string{"Terraform-Provisioner-foo"},
			expErr: "found provisioner plugins in",
		},
		{
			name:   "dot_terraform_file_rejected",
			files:  []string{".terraform"},
			expErr: "is not a directory",
		},
		{
			name:     "symlinked_dot_terraform_rejected",
			dirs:     []string{"elsewhere/providers"},
			symlinks: map[string]string{".terraform": "elsewhere"},
			expErr:   "is not a directory",
		},
		{
			name:   "plugins_rejected",
			dirs:   []string{".terraform/plugins/linux_amd64"},
			expErr: "found",
		},
		{
			name:   "plugins_is_case_insensitive",
			dirs:   []string{".terraform/Plugins"},
			expErr: "after terraform init",
		},
		{
			name:   "plugin_path_rejected",
			files:  []string{".terraform/plugin_path"},
			expErr: "after terraform init",
		},
		{
			name:   "providers_file_rejected",
			files:  []string{".terraform/providers"},
			expErr: "is not a directory",
		},
		{
			name:     "symlinked_providers_rejected",
			dirs:     []string{".terraform", "elsewhere"},
			symlinks: map[string]string{".terraform/providers": "../elsewhere"},
			expErr:   "is not a directory",
		},
		{
			name:     "symlinked_providers_is_case_insensitive",
			dirs:     []string{".terraform", "elsewhere"},
			symlinks: map[string]string{".terraform/Providers": "../elsewhere"},
			expErr:   "is not a directory",
		},
		{
			name:     "symlinked_modules_dot_rejected",
			dirs:     []string{".terraform/providers"},
			symlinks: map[string]string{".terraform/modules": "."},
			expErr:   "is not a directory",
		},
		{
			name:     "symlinked_modules_dotdot_rejected",
			dirs:     []string{".terraform"},
			symlinks: map[string]string{".terraform/modules": ".."},
			expErr:   "is not a directory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for _, d := range tc.dirs {
				mustMkdir(t, filepath.Join(dir, d))
			}
			for _, f := range tc.files {
				mustWrite(t, filepath.Join(dir, f), "")
			}
			for link, target := range tc.symlinks {
				if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
					t.Fatal(err)
				}
			}

			err := VerifyPluginDirsAfterInit(dir)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestCheckInstalledProviderSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		patterns     []string
		installed    []string // relative to .terraform/providers
		strayFiles   []string // relative to .terraform/providers
		noProviders  bool
		expInstalled []*ProviderSource
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
			expInstalled: []*ProviderSource{
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
			expInstalled: []*ProviderSource{
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
			expInstalled: []*ProviderSource{
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
			expInstalled: []*ProviderSource{
				{Hostname: "evil.example.com", Namespace: "hashicorp", Type: "google"},
			},
			expErr: "not allowed: evil.example.com/hashicorp/google",
		},
		{
			name:     "no_patterns_rejects_installed",
			patterns: nil,
			installed: []string{
				"registry.terraform.io/hashicorp/google/6.0.0/linux_amd64",
			},
			expInstalled: []*ProviderSource{
				{Hostname: "registry.terraform.io", Namespace: "hashicorp", Type: "google"},
			},
			expErr: `no provider sources are allowed; set -allowed-provider-sources to the exact providers you trust, e.g. -allowed-provider-sources=registry.terraform.io/hashicorp/google; "*" allows every provider, including ones that run arbitrary programs on the runner, and must only be used in test environments`,
		},
		{
			name:     "allow_all",
			patterns: []string{"*"},
			installed: []string{
				"registry.terraform.io/attacker/evil/1.0.0/linux_amd64",
				"registry.example.com/acme/thing/1.0.0/linux_amd64",
			},
			expInstalled: []*ProviderSource{
				{Hostname: "registry.example.com", Namespace: "acme", Type: "thing"},
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
			expInstalled: []*ProviderSource{
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
