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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseSortedDiffDirs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
		exp   []string
	}{
		{
			name: "success",
			value: `first/test.txt
second/test.txt
third/test.txt`,
			exp: []string{"first", "second", "third"},
		},
		{
			name:  "carriage_return_and_newline",
			value: "foo/test.txt\r\nbar/test.txt\r\nbaz/test.txt",
			exp:   []string{"bar", "baz", "foo"},
		},
		{
			name:  "sorts",
			value: "foo/test.txt\nbar/test.txt\nbaz/test.txt",
			exp:   []string{"bar", "baz", "foo"},
		},
		{
			name:  "handles_dirs",
			value: "test/first\ntest/second",
			exp:   []string{"test"},
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

			dirs := parseSortedDiffDirs(tc.value)
			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
