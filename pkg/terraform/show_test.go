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

	"github.com/abcxyz/guardian/pkg/util"
	"github.com/google/go-cmp/cmp"
)

func TestShowArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *ShowOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &ShowOptions{
				File:    util.Ptr[string]("filename"),
				NoColor: util.Ptr[bool](true),
				JSON:    util.Ptr[bool](true),
			},
			exp: []string{
				"show",
				"-no-color",
				"-json",
				"filename",
			},
		},
		{
			name: "falsey",
			opts: &ShowOptions{
				File:    util.Ptr[string]("filename"),
				NoColor: util.Ptr[bool](false),
				JSON:    util.Ptr[bool](false),
			},
			exp: []string{"show", "filename"},
		},
		{
			name: "empty",
			opts: &ShowOptions{},
			exp:  []string{"show"},
		},
		{
			name: "nil",
			opts: nil,
			exp:  []string{"show"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := showArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
