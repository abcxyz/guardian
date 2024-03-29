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
				Backend:     util.Ptr(true),
				Lock:        util.Ptr(true),
				LockTimeout: util.Ptr("10m"),
				Lockfile:    util.Ptr("readonly"),
				Input:       util.Ptr(true),
				NoColor:     util.Ptr(true),
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
				Backend:     util.Ptr(false),
				Lock:        util.Ptr(false),
				LockTimeout: util.Ptr("10m"),
				Lockfile:    util.Ptr("readonly"),
				Input:       util.Ptr(false),
				NoColor:     util.Ptr(false),
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
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := initArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
