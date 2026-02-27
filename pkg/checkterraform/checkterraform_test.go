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

func TestFindInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		allowed    []string
		disallowed []string
		actual     []string
		expResult  []string
	}{
		{
			name:       "An empty file should pass",
			allowed:    []string{},
			disallowed: []string{},
			actual:     []string{},
			expResult:  []string{},
		},
		{
			name:       "disallowed and actual set",
			allowed:    []string{"a", "b"},
			disallowed: []string{"c", "d"},
			actual:     []string{"a", "b", "c"},
			expResult:  []string{"c"},
		},
		{
			name:       "allowed == actual",
			allowed:    []string{"a", "b"},
			disallowed: []string{},
			actual:     []string{"a", "b"},
			expResult:  []string{},
		},
		{
			name:       "additional actual",
			allowed:    []string{"a", "b"},
			disallowed: []string{},
			actual:     []string{"a", "b", "c"},
			expResult:  []string{"c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := findInvalid(tc.allowed, tc.disallowed, tc.actual)
			if !cmp.Equal(result, tc.expResult) {
				t.Errorf("findInvalid() = %v, want %v", result, tc.expResult)
			}
		})
	}
}

func TestScan(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		filepath            string
		expResult           ScanResult
		expError            string
		allowedProviders    []string
		allowedProvisioners []string
	}{
		{
			name:      "An empty file should pass",
			filepath:  "testdata/empty.tf",
			expResult: ScanResult{},
		},
		{
			name:     "Only contains allowed providers",
			filepath: "testdata/valid.tf",
			expResult: ScanResult{
				Providers:           []string{"google", "random"},
				Provisioners:        []string{"safe-provisioner"},
				InvalidProviders:    []string{},
				InvalidProvisioners: []string{},
			},
		},
		{
			name:     "Should find an additional provider that isn't allowlisted",
			filepath: "testdata/disallowed_provider.tf",
			expResult: ScanResult{
				Providers:        []string{"aws", "external", "google", "random"},
				Provisioners:     nil,
				InvalidProviders: []string{"external"},
			},
			expError: "terraform contains invalid providers: [external], invalid provisioners: []",
		},
		{
			name:     "Should find an external resource that isn't allowlisted",
			filepath: "testdata/external_resource.tf",
			expResult: ScanResult{
				Providers:        []string{"external", "google", "random"},
				Provisioners:     nil,
				InvalidProviders: []string{"external"},
			},
			expError: "terraform contains invalid providers: [external], invalid provisioners: []",
		},
		{
			name:     "Should fail with a local_exec provisioner",
			filepath: "testdata/local_exec.tf",
			expResult: ScanResult{
				Providers:           []string{"google", "random"},
				Provisioners:        []string{"local-exec"},
				InvalidProviders:    []string{},
				InvalidProvisioners: []string{"local-exec"},
			},
			expError: "terraform contains invalid providers: [], invalid provisioners: [local-exec]",
		},
		{
			name:     "Should fail with an external data block",
			filepath: "testdata/external_data_output.tf",
			expResult: ScanResult{
				Providers:        []string{"external"},
				Provisioners:     nil,
				InvalidProviders: []string{"external"},
			},
			expError: "terraform contains invalid providers: [external], invalid provisioners: []",
		},
		{
			name:     "Should fail with a disallowed provider",
			filepath: "testdata/main.tf.json",
			expResult: ScanResult{
				Providers:        []string{"aws", "azurerm", "disallowed", "google", "random"},
				Provisioners:     nil,
				InvalidProviders: []string{"disallowed"},
			},
			expError: "terraform contains invalid providers: [disallowed], invalid provisioners: []",
		},
		{
			name:                "Validate that when we set allowed providers and allowed provisioners they override allowlist",
			filepath:            "testdata/valid.tf",
			allowedProviders:    []string{"google"},
			allowedProvisioners: []string{"some-provisioner"},
			expResult: ScanResult{
				Providers:           []string{"google", "random"},
				Provisioners:        []string{"safe-provisioner"},
				InvalidProviders:    []string{"random"},
				InvalidProvisioners: []string{"safe-provisioner"},
			},
			expError: "terraform contains invalid providers: [random], invalid provisioners: [safe-provisioner]",
		},
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := CheckProvidersProvisioners(context.Background(), filepath.Join(cwd, tc.filepath), []string{"external", "disallowed"}, []string{"local-exec", "remote-exec"}, tc.allowedProviders, tc.allowedProvisioners)
			if tc.expError != "" {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if diff := cmp.Diff(tc.expError, err.Error()); diff != "" {
					t.Errorf("Scan() error mismatch (-want +got):\n%s", diff)
				}
			} else if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}

			if diff := cmp.Diff(tc.expResult, result); diff != "" {
				t.Errorf("Scan() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
