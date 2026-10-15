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

	"github.com/abcxyz/pkg/testutil"
)

func TestExtractPathsFromModulesJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		modulesJSON *string
		modulesDir  bool
		expResult   []string
		expErr      string
	}{
		{
			name:      "missing modules.json results in just basedir",
			expResult: []string{""},
		},
		{
			name:        "empty file fails closed",
			modulesJSON: ptr(""),
			expErr:      "failed to parse modules.json",
		},
		{
			name:        "invalid file fails closed",
			modulesJSON: ptr(`not json`),
			expErr:      "failed to parse modules.json",
		},
		{
			name:       "unreadable file fails closed",
			modulesDir: true,
			expErr:     "failed to read modules.json",
		},
		{
			name: "discovers all directories",
			modulesJSON: ptr(`{
				"Modules": [
					{"Key": "", "Source": "", "Dir": "."},
					{"Key": "child", "Source": "./modules/child", "Dir": "modules/child"}
				]
			}`),
			expResult: []string{"", "modules/child"},
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
			if tc.modulesJSON != nil {
				if err := os.WriteFile(filepath.Join(modulesDir, "modules.json"), []byte(*tc.modulesJSON), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.modulesDir {
				if err := os.Mkdir(filepath.Join(modulesDir, "modules.json"), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			result, err := extractPathsFromModulesJSON(context.Background(), rootDir)
			if diff := testutil.DiffErrString(err, tc.expErr); diff != "" {
				t.Fatal(diff)
			}

			var expResult []string
			for _, path := range tc.expResult {
				expResult = append(expResult, filepath.Join(rootDir, path))
			}
			if diff := cmp.Diff(expResult, result); diff != "" {
				t.Errorf("extractPathsFromModulesJSON() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
