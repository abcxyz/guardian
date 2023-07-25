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

package util

import (
	"testing"

	"github.com/abcxyz/pkg/testutil"
)

func TestChildPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		base   string
		target string
		exp    string
		err    string
	}{
		{
			name:   "success",
			base:   ".",
			target: "./terraform/project",
			exp:    "terraform/project",
		},
		{
			name:   "empty",
			base:   "",
			target: "",
			exp:    "",
		},
		{
			name:   "not_child_dir",
			base:   "./terraform/project",
			target: ".",
			err:    "is not a child of",
		},
		{
			name:   "path_with_spaces",
			base:   ".",
			target: "./terraform/    /project",
			exp:    "terraform/    /project",
		},
		{
			name:   "path_with_special_chars",
			base:   ".",
			target: "./terraform/!/&/@/#/$/%/^/&/*/(/)/_/+/project",
			exp:    "terraform/!/&/@/#/$/%/^/&/*/(/)/_/+/project",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, err := ChildPath(tc.base, tc.target)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if got, want := d, tc.exp; got != want {
				t.Errorf("expected %s to be %s", got, want)
			}
		})
	}
}

func TestPathEvalAbs(t *testing.T) {
	t.Parallel()

	dir, err := PathEvalAbs(".")
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Errorf("expected dir to be defined")
	}
}
