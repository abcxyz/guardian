// Copyright 2023 The Authors (see AUTHORS file)
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

package terraform

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestFormatOutputForGitHubDiff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		exp     string
	}{
		{
			name: "replaces_tilde",
			content: `
first section -
first section +
first section ~
first section !

    second section -
    second section +
    second section ~
    second section !
	
- third section
+ third section
~ third section
! third section
	
    - fourth section
    + fourth section
    -/+ fourth section
    +/- fourth section
    ~ fourth section
    ! fourth section`,
			exp: `
first section -
first section +
first section ~
first section !

    second section -
    second section +
    second section ~
    second section !
	
- third section
+ third section
! third section
! third section
	
-     fourth section
+     fourth section
-/+     fourth section
+/-     fourth section
!     fourth section
!     fourth section`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := FormatOutputForGitHubDiff(tc.content)
			if got, want := strings.TrimSpace(output), strings.TrimSpace(tc.exp); got != want {
				t.Errorf("expected\n\n%s\n\nto be\n\n%s\n\n", got, want)
			}
		})
	}
}

func TestGetEntrypointDirectories(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  string
		exp  []*TerraformEntrypoint
		err  string
	}{
		{
			name: "has_backend",
			dir:  "testdata/backends",
			exp: []*TerraformEntrypoint{
				{Path: path.Join(cwd, "testdata/backends/project1"), BackendFile: path.Join(cwd, "testdata/backends/project1/main.tf")},
				{Path: path.Join(cwd, "testdata/backends/project2"), BackendFile: path.Join(cwd, "testdata/backends/project2/main.tf")},
			},
		},
		{
			name: "no_backend",
			dir:  "testdata/no-backends",
			exp:  []*TerraformEntrypoint{},
		},
		{
			name: "missing_directory",
			dir:  "testdata/missing",
			exp:  nil,
			err:  "no such file or directory",
		},
		{
			name: "empty",
			dir:  "",
			err:  "no such file or directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dirs, err := GetEntrypointDirectories(tc.dir)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Errorf("directories differed from expected, (-got,+want): %s", diff)
			}
		})
	}
}

func TestHasBackendConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		exp  bool
		err  string
	}{
		{
			name: "has_backend",
			file: "../../terraform/terraform.tf", // depend on test data in [REPO_ROOT]/terraform
			exp:  true,
		},
		{
			name: "no_backend",
			file: "../../terraform/main.tf", // depend on test data in [REPO_ROOT]/terraform
			exp:  false,
		},
		{
			name: "missing_file",
			file: "../../terraform/missing.tf", // depend on test data in [REPO_ROOT]/terraform
			exp:  false,
			err:  "failed to read file: ../../terraform/missing.tf",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found, _, err := hasBackendConfig(tc.file)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if got, want := found, tc.exp; got != want {
				t.Errorf("expected %t to be %t", got, want)
			}
		})
	}
}

func TestExtractBackendConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		want *TerraformBackendConfig
		err  string
	}{
		{
			name: "has_backend",
			file: "../../terraform/terraform.tf", // depend on test data in [REPO_ROOT]/terraform
			want: &TerraformBackendConfig{GCSBucket: util.Ptr("guardian-i-terraform-state-576047"), Prefix: util.Ptr("state/test")},
		},
		{
			name: "no_backend",
			file: "../../terraform/main.tf", // depend on test data in [REPO_ROOT]/terraform
			want: nil,
		},
		{
			name: "missing_file",
			file: "../../terraform/missing.tf", // depend on test data in [REPO_ROOT]/terraform
			want: nil,
			err:  "failed to read file: open ../../terraform/missing.tf: no such file or directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _, err := ExtractBackendConfig(tc.file)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("StateFileURIs() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_extractBackendConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
		want *TerraformBackendConfig
		err  string
	}{
		{
			name: "gcs_backend",
			data: []byte(`
				terraform {
				  backend "gcs" {
					bucket = "guardian-i-terraform-state-576047"
					prefix = "state/test"
				  }
				}`),
			want: &TerraformBackendConfig{GCSBucket: util.Ptr("guardian-i-terraform-state-576047"), Prefix: util.Ptr("state/test")},
		},
		{
			name: "local_backend",
			data: []byte(`
			terraform {
			  backend "local" {
				path = "/tmp/my/made/up/path"
			  }
			}`),
			want: &TerraformBackendConfig{},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _, err := extractBackendConfig(tc.data, "filename.tf")
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("StateFileURIs() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_modules(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  string
		exp  map[string]*Modules
		err  string
	}{
		{
			name: "has_modules",
			dir:  "testdata/with-modules",
			exp: map[string]*Modules{
				path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): {
					ModulePaths:        map[string]struct{}{path.Join(cwd, "testdata/with-modules/modules/module-a"): {}},
					ModuleOrEntrypoint: path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"),
				},
				path.Join(cwd, "testdata/with-modules/project1"): {
					ModulePaths: map[string]struct{}{
						path.Join(cwd, "testdata/with-modules/modules/module-a"):         {},
						path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): {},
					},
					ModuleOrEntrypoint: path.Join(cwd, "testdata/with-modules/project1"),
				},
				path.Join(cwd, "testdata/with-modules/project2"): {
					ModulePaths:        map[string]struct{}{path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): {}},
					ModuleOrEntrypoint: path.Join(cwd, "testdata/with-modules/project2"),
				},
			},
		},
		{
			name: "no_modules",
			dir:  "testdata/no-backends",
			exp:  map[string]*Modules{},
		},
		{
			name: "missing_directory",
			dir:  "testdata/missing",
			exp:  nil,
			err:  "no such file or directory",
		},
		{
			name: "empty",
			dir:  "",
			err:  "no such file or directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dirs, err := modules(tc.dir)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Errorf("directories differed from expected, (-got,+want): %s", diff)
			}
		})
	}
}

func TestModuleUsage(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dir  string
		exp  *ModuleUsageGraph
		err  string
	}{
		{
			name: "has_modules",
			dir:  "testdata/with-modules",
			exp: &ModuleUsageGraph{
				EntrypointToModules: map[string]map[string]struct{}{
					path.Join(cwd, "testdata/with-modules/project1"): {
						path.Join(cwd, "testdata/with-modules/modules/module-a"):         struct{}{},
						path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): struct{}{},
					},
					path.Join(cwd, "testdata/with-modules/project2"): {
						path.Join(cwd, "testdata/with-modules/modules/module-a"):         struct{}{},
						path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): struct{}{},
					},
					path.Join(cwd, "testdata/with-modules/project3"): {},
				},
				ModulesToEntrypoints: map[string]map[string]struct{}{
					path.Join(cwd, "testdata/with-modules/modules/module-a"): {
						path.Join(cwd, "testdata/with-modules/project1"): struct{}{},
						path.Join(cwd, "testdata/with-modules/project2"): struct{}{},
					},
					path.Join(cwd, "testdata/with-modules/modules/module-b-using-a"): {
						path.Join(cwd, "testdata/with-modules/project1"): struct{}{},
						path.Join(cwd, "testdata/with-modules/project2"): struct{}{},
					},
				},
			},
		},
		{
			name: "no_modules",
			dir:  "testdata/no-backends",
			exp: &ModuleUsageGraph{
				EntrypointToModules:  map[string]map[string]struct{}{},
				ModulesToEntrypoints: map[string]map[string]struct{}{},
			},
		},
		{
			name: "missing_directory",
			dir:  "testdata/missing",
			exp:  nil,
			err:  "no such file or directory",
		},
		{
			name: "empty",
			dir:  "",
			err:  "no such file or directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph, err := ModuleUsage(tc.dir)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(graph, tc.exp); diff != "" {
				t.Errorf("directories differed from expected, (-got,+want): %s", diff)
			}
		})
	}
}
