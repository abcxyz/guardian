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

package registryproxy

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestGenerateCLIConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		proxyURL     string
		wantEmpty    bool
		wantContains []string
	}{
		{
			name:      "empty_url",
			proxyURL:  "",
			wantEmpty: true,
		},
		{
			name:     "mirrors_all_providers",
			proxyURL: "https://localhost:8080",
			wantContains: []string{
				`url     = "https://localhost:8080/"`,
				`include = ["*/*/*"]`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, cleanup, err := GenerateCLIConfig(context.Background(), tc.proxyURL, "")
			if err != nil {
				t.Fatalf("GenerateCLIConfig() got err %v, want nil", err)
			}
			defer cleanup()

			if tc.wantEmpty {
				if path != "" {
					t.Errorf("GenerateCLIConfig() path got %q, want empty string", path)
				}
				return
			}
			if path == "" {
				t.Fatal("GenerateCLIConfig() path got empty string, want non-empty path")
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Stat() got err %v, want nil", err)
			}

			if runtime.GOOS != "windows" {
				if perm := info.Mode().Perm(); perm != 0o600 {
					t.Errorf("Perm() got %v, want 0o600", perm)
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() got err %v, want nil", err)
			}

			content := string(data)
			for _, want := range tc.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("GenerateCLIConfig() content got:\n%s\nwant substring %q", content, want)
				}
			}
		})
	}
}

func TestGenerateCLIConfig_NonHTTPSURL(t *testing.T) {
	t.Parallel()

	for _, proxyURL := range []string{"http://localhost:8080", "localhost:8080"} {
		_, cleanup, err := GenerateCLIConfig(context.Background(), proxyURL, "")
		cleanup()
		if err == nil {
			t.Errorf("GenerateCLIConfig(%q) got nil err, want error for non-https URL", proxyURL)
		}
	}
}

func TestGenerateCLIConfig_MergesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.tfrc")
	existingContent := "plugin_cache_dir = \"/tmp/plugin-cache\"\ndisable_checkpoint = true\n"
	if err := os.WriteFile(existing, []byte(existingContent), 0o600); err != nil {
		t.Fatalf("WriteFile() got err %v, want nil", err)
	}
	t.Setenv("TF_CLI_CONFIG_FILE", existing)

	path, cleanup, err := GenerateCLIConfig(context.Background(), "https://localhost:8080", "")
	if err != nil {
		t.Fatalf("GenerateCLIConfig() got err %v, want nil", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() got err %v, want nil", err)
	}

	content := string(data)
	for _, want := range []string{
		`plugin_cache_dir = "/tmp/plugin-cache"`,
		"disable_checkpoint = true",
		`url     = "https://localhost:8080/"`,
		`include = ["*/*/*"]`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("GenerateCLIConfig() content got:\n%s\nwant substring %q", content, want)
		}
	}
}

func TestGenerateCLIConfig_ResolvesRelativeExistingAgainstWorkingDir(t *testing.T) {
	// A relative TF_CLI_CONFIG_FILE must be resolved against the Terraform working
	// directory (workingDir), not the current process working directory, to match how
	// Terraform itself resolves it. Guardian runs Terraform with workingDir as its cwd.
	workingDir := t.TempDir()
	existingContent := `plugin_cache_dir = "/tmp/plugin-cache"` + "\n"
	if err := os.WriteFile(filepath.Join(workingDir, "existing.tfrc"), []byte(existingContent), 0o600); err != nil {
		t.Fatalf("WriteFile() got err %v, want nil", err)
	}
	// Deliberately a relative path; it only resolves correctly against workingDir.
	t.Setenv("TF_CLI_CONFIG_FILE", "existing.tfrc")

	path, cleanup, err := GenerateCLIConfig(context.Background(), "https://localhost:8080", workingDir)
	if err != nil {
		t.Fatalf("GenerateCLIConfig() got err %v, want nil", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() got err %v, want nil", err)
	}

	content := string(data)
	for _, want := range []string{
		`plugin_cache_dir = "/tmp/plugin-cache"`,
		`url     = "https://localhost:8080/"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("GenerateCLIConfig() content got:\n%s\nwant substring %q", content, want)
		}
	}
}

func TestGenerateCLIConfig_ConflictingProviderInstallation(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.tfrc")
	existingContent := "provider_installation {\n  direct {}\n}\n"
	if err := os.WriteFile(existing, []byte(existingContent), 0o600); err != nil {
		t.Fatalf("WriteFile() got err %v, want nil", err)
	}
	t.Setenv("TF_CLI_CONFIG_FILE", existing)

	_, cleanup, err := GenerateCLIConfig(context.Background(), "https://localhost:8080", "")
	defer cleanup()
	if err == nil {
		t.Fatal("GenerateCLIConfig() got nil err, want error for existing provider_installation block")
	}
	if !strings.Contains(err.Error(), "provider_installation") {
		t.Errorf("GenerateCLIConfig() err got %q, want it to mention provider_installation", err)
	}
}

func TestGenerateCLIConfig_ExistingConfigWithoutTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.tfrc")
	// Deliberately omit the trailing newline to exercise the separator logic that
	// keeps the existing directive from being glued onto the appended block.
	existingContent := `plugin_cache_dir = "/tmp/plugin-cache"`
	if err := os.WriteFile(existing, []byte(existingContent), 0o600); err != nil {
		t.Fatalf("WriteFile() got err %v, want nil", err)
	}
	t.Setenv("TF_CLI_CONFIG_FILE", existing)

	path, cleanup, err := GenerateCLIConfig(context.Background(), "https://localhost:8080", "")
	if err != nil {
		t.Fatalf("GenerateCLIConfig() got err %v, want nil", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() got err %v, want nil", err)
	}

	// The merged output must remain valid HCL. Without newline normalization the
	// existing directive and the appended block would run together on one line.
	if _, diags := hclparse.NewParser().ParseHCL(data, filepath.Base(path)); diags.HasErrors() {
		t.Errorf("ParseHCL() got diagnostics %v, want none; merged content:\n%s", diags, data)
	}

	content := string(data)
	for _, want := range []string{
		`plugin_cache_dir = "/tmp/plugin-cache"`,
		`url     = "https://localhost:8080/"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("GenerateCLIConfig() content got:\n%s\nwant substring %q", content, want)
		}
	}
}

func TestGenerateCLIConfig_UnparseableExistingConfig(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.tfrc")
	// An unterminated block is not valid HCL, so parsing fails and the merge must
	// fail closed rather than emit a possibly-invalid config.
	existingContent := "broken {"
	if err := os.WriteFile(existing, []byte(existingContent), 0o600); err != nil {
		t.Fatalf("WriteFile() got err %v, want nil", err)
	}
	t.Setenv("TF_CLI_CONFIG_FILE", existing)

	_, cleanup, err := GenerateCLIConfig(context.Background(), "https://localhost:8080", "")
	defer cleanup()
	if err == nil {
		t.Fatal("GenerateCLIConfig() got nil err, want error for unparseable existing config")
	}
}
