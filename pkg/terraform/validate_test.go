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

	"github.com/abcxyz/pkg/pointer"
)

func TestValidateArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *ValidateOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &ValidateOptions{
				NoColor: pointer.To(true),
				JSON:    pointer.To(true),
			},
			exp: []string{
				"-no-color",
				"-json",
			},
		},
		{
			name: "falsey",
			opts: &ValidateOptions{
				NoColor: pointer.To(false),
				JSON:    pointer.To(false),
			},
			exp: []string{},
		},
		{
			name: "empty",
			opts: &ValidateOptions{},
			exp:  []string{},
		},
		{
			name: "nil",
			opts: nil,
			exp:  []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := validateArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Error(diff)
			}
		})
	}
}
