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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/abcxyz/pkg/logging"
)

// dataDirName is the name of the Terraform data directory in the working
// directory. terraform init writes provider packages, module packages,
// backend configuration and plugin settings here.
const dataDirName = ".terraform"

// terraformDName is the name of the implied local plugin directory in the
// working directory. Terraform treats terraform.d/plugins as a local filesystem
// mirror that takes precedence over the registry, and also searches it for
// provisioner plugins.
const terraformDName = "terraform.d"

// providersDirName and modulesDirName are the directories, relative to the
// Terraform data directory, where terraform init installs provider and module
// packages.
const (
	providersDirName = "providers"
	modulesDirName   = "modules"
)

// legacyPluginsDirName is the legacy provider cache directory, relative to the
// Terraform data directory. Terraform still searches
// .terraform/plugins/<os_arch> for provisioner plugins, and current versions
// of terraform init never create it.
const legacyPluginsDirName = "plugins"

// pluginPathFile is the file, relative to the Terraform data directory, that
// terraform init -plugin-dir writes. When it exists, every later Terraform
// command (validate, plan, apply, ...) searches only the directories it lists
// for provisioner plugins, so an untrusted copy can point Terraform at any
// binary. terraform init without -plugin-dir does not remove it.
const pluginPathFile = "plugin_path"

// provisionerPluginPrefix is the file name prefix of a provisioner plugin.
// Terraform searches the working directory for files with this prefix, and a
// discovered plugin overrides a built-in provisioner of the same name.
const provisionerPluginPrefix = "terraform-provisioner-"

// RemoveLocalPluginOverrides prepares dir so that Terraform can only install
// providers from their declared sources and cannot load provisioner plugins
// from the (possibly untrusted) working directory. It must be called before
// terraform init, and [VerifyPluginDirsAfterInit] must be called after it.
//
//   - terraform.d: Terraform treats terraform.d/plugins in the working
//     directory as an implied local filesystem mirror that takes precedence over
//     the registry, and also searches it for provisioner plugins. Terraform
//     never creates it, so its presence is an error.
//   - terraform-provisioner-*: Terraform searches the working directory for
//     provisioner plugins, which run during terraform validate and override
//     built-in provisioners such as local-exec. Terraform never creates these,
//     so their presence is an error.
//   - .terraform: the whole Terraform data directory is removed, whether it is
//     a directory, a file or a symlink, so terraform init re-creates it from the
//     configuration. Selectively removing parts of it is not safe. For example,
//     terraform init reuses cached provider packages that match the dependency
//     lock file (which the same change controls), loads plugins from any
//     directory in plugin_path, follows a symlinked .terraform/modules (letting
//     a module package be installed over .terraform/providers or as
//     terraform.d), and reuses cached backend configuration, including
//     -backend-config values that are not in the configuration.
//
// Names are matched case-insensitively, since Terraform may resolve them on a
// case-insensitive file system. Symlinks are removed, never followed.
func RemoveLocalPluginOverrides(ctx context.Context, dir string) error {
	logger := logging.FromContext(ctx)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", dir, err)
	}
	if err := checkWorkingDirPlugins(dir, entries); err != nil {
		return err
	}

	// Remove .terraform even if ReadDir did not list it under that exact name
	// (for example, on a file system that folds case differently).
	remove := []string{filepath.Join(dir, dataDirName)}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), dataDirName) && e.Name() != dataDirName {
			remove = append(remove, filepath.Join(dir, e.Name()))
		}
	}
	for _, p := range remove {
		if _, err := os.Lstat(p); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to check for %q: %w", p, err)
		}
		// os.RemoveAll does not follow symlinks, so a symlink only removes the
		// link itself.
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("failed to remove %q: %w", p, err)
		}
		logger.InfoContext(ctx, "removed terraform data directory so plugins are only loaded from trusted locations",
			"path", p)
	}
	return nil
}

// VerifyPluginDirsAfterInit checks that terraform init did not leave dir in a
// state where Terraform could load providers or provisioner plugins from an
// untrusted location. It must be called after terraform init and before any
// Terraform command that may execute providers or provisioners. It rejects:
//
//   - terraform.d or terraform-provisioner-* in dir, which terraform init never
//     creates (see [RemoveLocalPluginOverrides]).
//   - a .terraform that is not a real directory (for example, a symlink).
//   - .terraform/plugins or .terraform/plugin_path, which terraform init (as
//     run by Guardian) never creates.
//   - a .terraform/providers or .terraform/modules that is not a real
//     directory.
//
// Names are matched case-insensitively.
func VerifyPluginDirsAfterInit(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", dir, err)
	}
	if err := checkWorkingDirPlugins(dir, entries); err != nil {
		return err
	}

	dataDir := filepath.Join(dir, dataDirName)
	info, err := os.Lstat(dataDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check for %q: %w", dataDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to run: %q is not a directory", dataDir)
	}

	dataEntries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", dataDir, err)
	}
	for _, e := range dataEntries {
		p := filepath.Join(dataDir, e.Name())
		switch strings.ToLower(e.Name()) {
		case legacyPluginsDirName, pluginPathFile:
			return fmt.Errorf("refusing to run: found %q after terraform init; it can make Terraform "+
				"load plugins from untrusted locations", p)
		case providersDirName, modulesDirName:
			info, err := os.Lstat(p)
			if err != nil {
				return fmt.Errorf("failed to check for %q: %w", p, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("refusing to run: %q is not a directory", p)
			}
		}
	}
	return nil
}

// checkWorkingDirPlugins returns an error if dir contains terraform.d or a
// terraform-provisioner-* plugin (matched case-insensitively). entries are
// the entries of dir.
func checkWorkingDirPlugins(dir string, entries []os.DirEntry) error {
	var plugins []string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if name == terraformDName {
			return fmt.Errorf("refusing to run: found terraform.d in %q: %s; Terraform uses terraform.d/plugins "+
				"as a local provider mirror, which can substitute provider binaries", dir, e.Name())
		}
		if strings.HasPrefix(name, provisionerPluginPrefix) {
			plugins = append(plugins, e.Name())
		}
	}
	if len(plugins) > 0 {
		return fmt.Errorf("refusing to run: found provisioner plugins in %q: %s; Terraform loads "+
			"terraform-provisioner-* plugins from the working directory, which can run arbitrary code "+
			"and override built-in provisioners", dir, strings.Join(plugins, ", "))
	}
	return nil
}

// CheckInstalledProviderSources inspects the providers that terraform init
// installed into <dir>/.terraform/providers and returns an error if any of
// them is not allowed by m. It returns the list of installed providers.
//
// This is the authoritative provider gate and must be called after terraform
// init and before any command that executes providers (validate, plan, apply,
// etc.). If m has no patterns, every installed provider is rejected.
func CheckInstalledProviderSources(ctx context.Context, dir string, m *SourceMatcher) ([]*ProviderSource, error) {
	logger := logging.FromContext(ctx)

	installed, err := installedProviders(filepath.Join(dir, ".terraform", "providers"))
	if err != nil {
		return nil, fmt.Errorf("failed to list installed providers: %w", err)
	}

	logger.DebugContext(ctx, "found installed terraform providers",
		"dir", dir,
		"providers", installed)

	var disallowed []string
	for _, p := range installed {
		if !m.Allowed(p) {
			disallowed = append(disallowed, p.String())
		}
	}

	if len(disallowed) > 0 {
		return installed, newDisallowedSourcesError(disallowed, m)
	}
	return installed, nil
}

// installedProviders walks a Terraform provider install directory, which has
// the layout <root>/<hostname>/<namespace>/<type>/<version>/<os_arch>, and
// returns the sorted list of providers it contains. A missing root is not an
// error and returns no providers.
func installedProviders(root string) ([]*ProviderSource, error) {
	hosts, err := subdirs(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []*ProviderSource
	for _, host := range hosts {
		namespaces, err := subdirs(filepath.Join(root, host))
		if err != nil {
			return nil, err
		}
		for _, ns := range namespaces {
			types, err := subdirs(filepath.Join(root, host, ns))
			if err != nil {
				return nil, err
			}
			for _, typ := range types {
				out = append(out, &ProviderSource{
					Hostname:  strings.ToLower(host),
					Namespace: strings.ToLower(ns),
					Type:      strings.ToLower(typ),
				})
			}
		}
	}

	slices.SortFunc(out, func(a, b *ProviderSource) int {
		return strings.Compare(a.String(), b.String())
	})
	return slices.CompactFunc(out, func(a, b *ProviderSource) bool {
		return *a == *b
	}), nil
}

// subdirs returns the names of the directories (following symlinks) directly
// inside dir. Non-directory entries are ignored.
func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", dir, err)
	}

	var out []string
	for _, e := range entries {
		info, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to stat %q: %w", filepath.Join(dir, e.Name()), err)
		}
		if info.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func newDisallowedSourcesError(disallowed []string, m *SourceMatcher) error {
	slices.Sort(disallowed)
	disallowed = slices.Compact(disallowed)
	if len(m.Patterns()) == 0 {
		return fmt.Errorf("terraform uses provider sources that are not allowed: %s (no provider sources are allowed; "+
			"set -allowed-provider-sources, or use %q to allow all)",
			strings.Join(disallowed, ", "), AllowAllPattern)
	}
	return fmt.Errorf("terraform uses provider sources that are not allowed: %s (allowed provider sources: %s)",
		strings.Join(disallowed, ", "), strings.Join(m.Patterns(), ", "))
}

func newDisallowedProvisionerSourcesError(disallowed []string, m *ProvisionerSourceMatcher) error {
	slices.Sort(disallowed)
	disallowed = slices.Compact(disallowed)
	if len(m.Patterns()) == 0 {
		return fmt.Errorf("terraform uses provisioners that are not allowed: %s (no provisioners are allowed; "+
			"set -allowed-provisioner-sources, or use %q to allow all)",
			strings.Join(disallowed, ", "), AllowAllPattern)
	}
	return fmt.Errorf("terraform uses provisioners that are not allowed: %s (allowed provisioner sources: %s)",
		strings.Join(disallowed, ", "), strings.Join(m.Patterns(), ", "))
}
