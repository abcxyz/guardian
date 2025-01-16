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

func TestInitArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *InitOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &InitOptions{
				Backend:     pointer.To(true),
				Lock:        pointer.To(true),
				LockTimeout: pointer.To("10m"),
				Lockfile:    pointer.To("readonly"),
				Input:       pointer.To(true),
				NoColor:     pointer.To(true),
			},
			exp: []string{
				"-backend=true",
				"-input=true",
				"-no-color",
				"-lock=true",
				"-lock-timeout=10m",
				"-lockfile=readonly",
			},
		},
		{
			name: "falsey",
			opts: &InitOptions{
				Backend:     pointer.To(false),
				Lock:        pointer.To(false),
				LockTimeout: pointer.To("10m"),
				Lockfile:    pointer.To("readonly"),
				Input:       pointer.To(false),
				NoColor:     pointer.To(false),
			},
			exp: []string{
				"-backend=false",
				"-input=false",
				"-lock=false",
				"-lock-timeout=10m",
				"-lockfile=readonly",
			},
		},
		{
			name: "empty",
			opts: &InitOptions{},
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

			args := initArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Error(diff)
			}
		})
	}
}
