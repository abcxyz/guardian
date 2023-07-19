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

package git

import (
	"os"
	"path"
	"testing"

	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestParseSortedDiffDirsAbs(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		value string
		exp   []string
		err   string
	}{
		{
			name: "success",
			value: `testdata/first/test.txt
testdata/second/test.txt
testdata/third/test.txt`,
			exp: []string{
				path.Join(cwd, "testdata/first"),
				path.Join(cwd, "testdata/second"),
				path.Join(cwd, "testdata/third"),
			},
		},
		{
			name:  "carriage_return_and_newline",
			value: "testdata/first/test.txt\r\ntestdata/third/test.txt\r\ntestdata/second/test.txt",
			exp: []string{
				path.Join(cwd, "testdata/first"),
				path.Join(cwd, "testdata/second"),
				path.Join(cwd, "testdata/third"),
			},
		},
		{
			name: "handles_dirs",
			value: `testdata/first
testdata/second
testdata/third`,
			exp: []string{path.Join(cwd, "testdata")},
		},
		{
			name:  "handles_empty",
			value: "",
			exp:   []string{},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dirs, err := parseSortedDiffDirsAbs(tc.value)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
