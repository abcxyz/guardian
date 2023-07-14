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

func TestTerraform_applyArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *ApplyOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &ApplyOptions{
				File:            util.Ptr[string]("filename"),
				AutoApprove:     util.Ptr[bool](true),
				CompactWarnings: util.Ptr[bool](true),
				Lock:            util.Ptr[bool](true),
				LockTimeout:     util.Ptr[string]("10m"),
				Input:           util.Ptr[bool](true),
				NoColor:         util.Ptr[bool](true),
			},
			exp: []string{
				"apply",
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
				File:            util.Ptr[string]("filename"),
				AutoApprove:     util.Ptr[bool](false),
				CompactWarnings: util.Ptr[bool](false),
				Lock:            util.Ptr[bool](false),
				LockTimeout:     util.Ptr[string]("10m"),
				Input:           util.Ptr[bool](false),
				NoColor:         util.Ptr[bool](false),
			},
			exp: []string{
				"apply",
				"-lock=false",
				"-lock-timeout=10m",
				"-input=false",
				"filename",
			},
		},
		{
			name: "empty",
			opts: &ApplyOptions{},
			exp:  []string{"apply"},
		},
		{
			name: "nil",
			opts: nil,
			exp:  []string{"apply"},
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
