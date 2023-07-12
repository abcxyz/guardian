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
	"fmt"
	"strings"
	"testing"

	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestTerraform_FormatOutputForGitHubDiff(t *testing.T) {
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

func TestTerraform_GetEntrypointDirectories(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dir  string
		exp  []string
		err  string
	}{
		{
			name: "has_backend",
			dir:  "../../terraform", // depend on test data in [REPO_ROOT]/terraform
			exp:  []string{"../../terraform", "../../terraform/has-backend"},
		},
		{
			name: "no_backend",
			dir:  "../../terraform/no-backend", // depend on test data in [REPO_ROOT]/terraform
			exp:  []string{},
		},
		{
			name: "missing_directory",
			dir:  "../../terraform/missing", // depend on test data in [REPO_ROOT]/terraform
			exp:  nil,
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

func TestTerraform_hasBackendConfig(t *testing.T) {
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

			fmt.Printf("%+v", err)

			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if got, want := found, tc.exp; got != want {
				t.Errorf("expected %t to be %t", got, want)
			}
		})
	}
}