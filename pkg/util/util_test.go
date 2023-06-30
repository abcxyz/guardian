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

	"github.com/google/go-cmp/cmp"
)

func TestUtil_GetSliceIntersection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a    []string
		b    []string
		exp  []string
	}{
		{
			name: "success",
			a:    []string{"a", "b", "c", "d"},
			b:    []string{"b", "c"},
			exp:  []string{"b", "c"},
		},
		{
			name: "sorts",
			a:    []string{"d", "c", "b", "a"},
			b:    []string{"a", "d"},
			exp:  []string{"a", "d"},
		},
		{
			name: "handles_empty",
			a:    []string{"d", "d", "c", "d", "b", "a"},
			b:    []string{},
			exp:  []string{},
		},
		{
			name: "exclude_duplicates",
			a:    []string{"d", "d", "c", "d", "b", "a"},
			b:    []string{"a", "d", "a", "d"},
			exp:  []string{"a", "d"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := GetSliceIntersection(tc.a, tc.b)

			opts := []cmp.Option{}
			if diff := cmp.Diff(v, tc.exp, opts...); diff != "" {
				t.Errorf("got %#v, want %#v, diff (-got, +want): %v", v, tc.exp, diff)
			}
		})
	}
}
