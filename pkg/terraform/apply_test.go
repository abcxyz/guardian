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

func TestApplyArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *ApplyOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &ApplyOptions{
				File:            util.Ptr("filename"),
				AutoApprove:     util.Ptr(true),
				CompactWarnings: util.Ptr(true),
				Lock:            util.Ptr(true),
				LockTimeout:     util.Ptr("10m"),
				Input:           util.Ptr(true),
				NoColor:         util.Ptr(true),
			},
			exp: []string{
				"-auto-approve",
				"-compact-warnings",
				"-lock=true",
				"-lock-timeout=10m",
				"-input=true",
				"-no-color",
				"filename",
			},
		},
		{
			name: "falsey",
			opts: &ApplyOptions{
				File:            util.Ptr("filename"),
				AutoApprove:     util.Ptr(false),
				CompactWarnings: util.Ptr(false),
				Lock:            util.Ptr(false),
				LockTimeout:     util.Ptr("10m"),
				Input:           util.Ptr(false),
				NoColor:         util.Ptr(false),
			},
			exp: []string{
				"-lock=false",
				"-lock-timeout=10m",
				"-input=false",
				"filename",
			},
		},
		{
			name: "empty",
			opts: &ApplyOptions{},
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

			args := applyArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
