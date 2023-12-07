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

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/guardian/pkg/util"
)

func TestFormatArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *FormatOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &FormatOptions{
				Check:     util.Ptr(true),
				Diff:      util.Ptr(true),
				List:      util.Ptr(true),
				NoColor:   util.Ptr(true),
				Recursive: util.Ptr(true),
				Write:     util.Ptr(true),
			},
			exp: []string{
				"-check",
				"-diff",
				"-list=true",
				"-no-color",
				"-recursive",
				"-write=true",
			},
		},
		{
			name: "falsey",
			opts: &FormatOptions{
				Check:     util.Ptr(false),
				Diff:      util.Ptr(false),
				List:      util.Ptr(false),
				NoColor:   util.Ptr(false),
				Recursive: util.Ptr(false),
				Write:     util.Ptr(false),
			},
			exp: []string{
				"-list=false",
				"-write=false",
			},
		},
		{
			name: "empty",
			opts: &FormatOptions{},
			exp:  []string{},
		},
		{
			name: "nil",
			opts: nil,
			exp:  []string{},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := formatArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
