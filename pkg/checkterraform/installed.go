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

// providerCacheDirs are the directories, relative to the Terraform data
// directory (.terraform), where terraform init installs provider packages.
// terraform init reuses a package found here if it matches the dependency lock
// file, which is controlled by the same (possibly untrusted) change as the
// package itself, so these must be removed before init.
var providerCacheDirs = []string{"providers", "plugins"}

// RemoveLocalProviderOverrides prepares dir so that terraform init can only
// install providers from their declared sources. It must be called before
// terraform init.
//
//   - terraform.d: Terraform treats terraform.d/plugins in the working
//     directory as an implied local filesystem mirror that takes precedence over
//     the registry. Terraform never creates it, so its presence is an error.
//   - .terraform/providers (and the legacy .terraform/plugins): these provider
//     caches are removed so every provider is re-installed from its source and
//     verified against the lock file. The rest of .terraform (backend
//     configuration, selected workspace, modules) is left untouched so local
//     re-runs keep working.
func RemoveLocalProviderOverrides(ctx context.Context, dir string) error {
	logger := logging.FromContext(ctx)

	terraformD := filepath.Join(dir, "terraform.d")
	if _, err := os.Lstat(terraformD); err == nil {
		return fmt.Errorf("refusing to run: found terraform.d in %q; Terraform uses terraform.d/plugins "+
			"as a local provider mirror, which can substitute provider binaries", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to check for %q: %w", terraformD, err)
	}

	dataDir := filepath.Join(dir, ".terraform")
	info, err := os.Lstat(dataDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check for %q: %w", dataDir, err)
	}
	// Refuse to follow a symlinked or non-directory .terraform, otherwise removing
	// its contents could delete files outside of dir.
	if !info.IsDir() {
		return fmt.Errorf("refusing to run: %q is not a directory", dataDir)
	}

	for _, name := range providerCacheDirs {
		p := filepath.Join(dataDir, name)
		if _, err := os.Lstat(p); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to check for %q: %w", p, err)
		}

		// os.RemoveAll does not follow symlinks, so a symlinked cache only removes
		// the link itself.
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("failed to remove cached providers %q: %w", p, err)
		}
		logger.InfoContext(ctx, "removed cached terraform providers so they are re-installed from their sources",
			"path", p)
	}
	return nil
}

// CheckInstalledProviderSources inspects the providers that terraform init
// installed into <dir>/.terraform/providers and returns an error if any of
// them is not allowed by m. It returns the list of installed providers.
//
// This is the authoritative provider gate and must be called after terraform
// init and before any command that executes providers (validate, plan, apply,
// etc.). If m is not enabled, the installed providers are returned without
// being checked.
func CheckInstalledProviderSources(ctx context.Context, dir string, m *SourceMatcher) ([]ProviderSource, error) {
	logger := logging.FromContext(ctx)

	installed, err := installedProviders(filepath.Join(dir, ".terraform", "providers"))
	if err != nil {
		return nil, fmt.Errorf("failed to list installed providers: %w", err)
	}

	logger.DebugContext(ctx, "found installed terraform providers",
		"dir", dir,
		"providers", installed)

	if !m.Enabled() {
		return installed, nil
	}

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
func installedProviders(root string) ([]ProviderSource, error) {
	hosts, err := subdirs(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []ProviderSource
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
				out = append(out, ProviderSource{
					Hostname:  strings.ToLower(host),
					Namespace: strings.ToLower(ns),
					Type:      strings.ToLower(typ),
				})
			}
		}
	}

	slices.SortFunc(out, func(a, b ProviderSource) int {
		return strings.Compare(a.String(), b.String())
	})
	return slices.Compact(out), nil
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
	return fmt.Errorf("terraform uses provider sources that are not allowed: %s (allowed provider sources: %s)",
		strings.Join(disallowed, ", "), strings.Join(m.Patterns(), ", "))
}
