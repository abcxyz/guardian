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
			if diff := cmp.Diff(output, tc.exp); diff != "" {
				t.Errorf("got %#v, want %#v, diff (-got, +want): %v", output, tc.exp, diff)
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
			name: "success_has_backend",
			dir:  "../../terraform",
			exp:  []string{"../../terraform", "../../terraform/has-backend"},
		},
		{
			name: "success_missing_directory",
			dir:  "../../terraform/missing",
			exp:  nil,
			err:  "no such file or directory",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tfclient := NewTerraformClient()
			dirs, err := tfclient.GetEntrypointDirectories(tc.dir)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf("unexpected error: %v", diff)
			}

			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Errorf("directories differed from expected, (-got,+want): %s", diff)
			}
		})
	}
}
