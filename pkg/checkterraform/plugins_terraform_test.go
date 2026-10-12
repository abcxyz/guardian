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
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/abcxyz/guardian/pkg/checkterraform/plugintest"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

// requireTerraformEnv, when set to "true", makes the real Terraform tests fail
// instead of skip when terraform is not installed. CI sets it so these tests
// cannot be skipped silently.
const requireTerraformEnv = "GUARDIANTEST_REQUIRE_TERRAFORM"

// TestLocalProvisionerPlugins_Terraform runs real Terraform against the
// provisioner plugin fixtures. For each fixture it checks that:
//
//  1. Without Guardian's checks, terraform init and terraform validate execute
//     the plugin. This detects changes in Terraform's plugin discovery that
//     would make the fixtures (and Guardian's checks) stale.
//  2. With RemoveLocalPluginOverrides, the directory is rejected or cleaned so
//     the plugin is never executed.
func TestLocalProvisionerPlugins_Terraform(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("fake provisioner plugins are shell scripts")
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		if os.Getenv(requireTerraformEnv) == "true" {
			t.Fatalf("terraform is required when %s=true: %v", requireTerraformEnv, err)
		}
		t.Skipf("terraform not found on PATH: %v", err)
	}

	for _, tc := range plugintest.Cases() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			t.Run("without_guardian", func(t *testing.T) {
				t.Parallel()

				dir := t.TempDir()
				marker := filepath.Join(t.TempDir(), "marker")
				tc.Setup(t, dir, marker)

				validateErr := terraformInitValidate(t, dir)
				if got := exists(t, marker); got != tc.TerraformRunsPlugin {
					t.Errorf("plugin executed = %t, want %t (validate error: %v)", got, tc.TerraformRunsPlugin, validateErr)
				}
			})

			t.Run("with_guardian", func(t *testing.T) {
				t.Parallel()

				ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))
				dir := t.TempDir()
				marker := filepath.Join(t.TempDir(), "marker")
				tc.Setup(t, dir, marker)

				err := RemoveLocalPluginOverrides(ctx, dir)
				if tc.WantRejected {
					if diff := testutil.DiffErrString(err, plugintest.RejectedErr); diff != "" {
						t.Fatal(diff)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}

				if err := terraformInitValidate(t, dir); err != nil {
					t.Errorf("terraform validate failed: %v", err)
				}
				if exists(t, marker) {
					t.Errorf("plugin was executed")
				}
			})
		})
	}
}

// terraformInitValidate runs terraform init and terraform validate in dir with
// an empty CLI configuration, and returns the first error.
func terraformInitValidate(tb testing.TB, dir string) error {
	tb.Helper()

	cliConfig := filepath.Join(tb.TempDir(), "terraformrc")
	if err := os.WriteFile(cliConfig, nil, 0o600); err != nil {
		tb.Fatal(err)
	}

	for _, args := range [][]string{
		{"init", "-input=false", "-no-color", "-backend=false"},
		{"validate", "-no-color"},
	} {
		if err := runTerraform(tb, dir, cliConfig, args...); err != nil {
			return err
		}
	}
	return nil
}

func runTerraform(tb testing.TB, dir, cliConfig string, args ...string) error {
	tb.Helper()

	var out bytes.Buffer
	cmd := exec.CommandContext(tb.Context(), "terraform", args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Env = append(os.Environ(),
		"TF_CLI_CONFIG_FILE="+cliConfig,
		"TF_IN_AUTOMATION=true",
		"CHECKPOINT_DISABLE=1",
	)
	if err := cmd.Run(); err != nil {
		tb.Logf("terraform %v output:\n%s", args, out.String())
		return err
	}
	return nil
}

func exists(tb testing.TB, p string) bool {
	tb.Helper()

	_, err := os.Lstat(p)
	if err == nil {
		return true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		tb.Fatal(err)
	}
	return false
}
