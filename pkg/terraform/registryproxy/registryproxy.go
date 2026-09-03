// Copyright 2026 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package registryproxy generates Terraform CLI configuration for Guardian, such
// as a temporary .terraformrc that routes provider requests through a registry proxy.
package registryproxy

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/abcxyz/pkg/logging"
)

// GenerateCLIConfig creates a temporary Terraform CLI configuration file (.terraformrc)
// configured to route all provider requests through the Provider Network Mirror at proxyURL.
// The proxy is fully transparent: it decides where to fetch each provider from, so all
// providers ("*/*/*") are routed through it. proxyURL must use the https:// scheme,
// because Terraform only accepts https network mirror URLs.
//
// If TF_CLI_CONFIG_FILE is already set, its contents are preserved by prepending them to
// the generated config, so top-level directives (such as plugin_cache_dir or credentials)
// are not lost. Because Terraform permits only one provider_installation block, an existing
// config that already declares one cannot be merged and results in an error. A relative
// TF_CLI_CONFIG_FILE is resolved against workingDir, matching how Terraform resolves it
// (Guardian runs the Terraform subprocess with workingDir as its working directory).
//
// It returns the absolute file path, a cleanup function to remove the temporary file, and an error if any.
// An empty proxyURL returns an empty path and a no-op cleanup with a nil error. The returned cleanup is
// always non-nil and safe to call, even when an error is returned.
func GenerateCLIConfig(ctx context.Context, proxyURL, workingDir string) (string, func(), error) {
	if proxyURL == "" {
		return "", func() {}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return "", func() {}, fmt.Errorf("invalid registry proxy URL %q: %w", proxyURL, err)
	}
	if u.Scheme != "https" {
		return "", func() {}, fmt.Errorf("registry proxy URL %q must use https:// (Terraform requires network mirror URLs to use https)", proxyURL)
	}

	if !strings.HasSuffix(proxyURL, "/") {
		proxyURL += "/"
	}

	var buf bytes.Buffer

	if existing := os.Getenv("TF_CLI_CONFIG_FILE"); existing != "" {
		// Terraform resolves a relative TF_CLI_CONFIG_FILE against its working
		// directory. Guardian runs Terraform with workingDir as that directory, so
		// resolve it the same way here to read the file Terraform would have used.
		if !filepath.IsAbs(existing) {
			existing = filepath.Join(workingDir, existing)
		}
		data, err := os.ReadFile(existing)
		if err != nil {
			return "", func() {}, fmt.Errorf("failed to read existing TF_CLI_CONFIG_FILE %q: %w", existing, err)
		}
		hasBlock, err := hasProviderInstallationBlock(data)
		if err != nil {
			return "", func() {}, fmt.Errorf("failed to parse existing TF_CLI_CONFIG_FILE %q: %w", existing, err)
		}
		if hasBlock {
			return "", func() {}, fmt.Errorf("existing TF_CLI_CONFIG_FILE %q declares a provider_installation block, which conflicts with --registry-proxy (Terraform permits only one); remove it or unset TF_CLI_CONFIG_FILE", existing)
		}
		logging.FromContext(ctx).InfoContext(ctx, "Merging existing TF_CLI_CONFIG_FILE with registry proxy configuration", "existing_tf_cli_config_file", existing)
		buf.Write(data)
		if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}

	fmt.Fprintf(&buf, `provider_installation {
  network_mirror {
    url     = %q
    include = ["*/*/*"]
  }
}
`, proxyURL)

	tmpFile, err := os.CreateTemp("", "guardian-terraformrc-*.hcl")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp terraformrc: %w", err)
	}

	if err := os.Chmod(tmpFile.Name(), 0o600); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to chmod temp terraformrc: %w", err)
	}

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to write temp terraformrc: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to close temp terraformrc: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}

// hasProviderInstallationBlock reports whether the given Terraform CLI config source already declares a provider_installation block.
// It returns an error if the source cannot be parsed, so callers can distinguish a genuine conflicting block from an unparseable config.
func hasProviderInstallationBlock(src []byte) (bool, error) {
	f, diags := hclparse.NewParser().ParseHCL(src, "existing.terraformrc")
	if diags.HasErrors() {
		return false, fmt.Errorf("parsing HCL: %w", diags)
	}
	if f == nil {
		return false, fmt.Errorf("parsing HCL: parser returned no file")
	}
	content, _, _ := f.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "provider_installation"}},
	})
	return content != nil && len(content.Blocks) > 0, nil
}
