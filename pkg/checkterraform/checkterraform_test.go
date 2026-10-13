// Copyright 2026 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkterraform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExtractPathsFromModulesJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		modulesJSON string
		expResult   []string
	}{
		{
			name:        "Empty JSON results in just basedir.",
			modulesJSON: "",
			expResult:   []string{""},
		},
		{
			name:        "Should fail gracefully if file invalid, still returning the basedir.",
			modulesJSON: `not json`,
			expResult:   []string{""},
		},
		{
			name: "Should discover all directories.",
			modulesJSON: `{
				"Modules": [
					{"Key": "", "Source": "", "Dir": "."},
					{"Key": "child", "Source": "./modules/child", "Dir": "modules/child"}
				]
			}`,
			expResult: []string{".", "modules/child"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			modulesDir := filepath.Join(rootDir, ".terraform", "modules")
			if err := os.MkdirAll(modulesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(modulesDir, "modules.json"), []byte(tc.modulesJSON), 0o600); err != nil {
				t.Fatal(err)
			}

			result := extractPathsFromModulesJSON(context.Background(), rootDir)
			if result == nil {
				t.Fatal("expected result, got nil")
			}

			expResult := tc.expResult
			for i, path := range expResult {
				expResult[i] = filepath.Join(rootDir, path)
			}
			if diff := cmp.Diff(expResult, result); diff != "" {
				t.Errorf("extractPathsFromModulesJSON() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
