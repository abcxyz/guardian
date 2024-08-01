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

	"github.com/abcxyz/pkg/pointer"
	"github.com/google/go-cmp/cmp"
)

func TestPlanArgsFromOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts *PlanOptions
		exp  []string
	}{
		{
			name: "truthy",
			opts: &PlanOptions{
				CompactWarnings:  pointer.To(true),
				Destroy:          pointer.To(true),
				DetailedExitcode: pointer.To(true),
				Lock:             pointer.To(true),
				LockTimeout:      pointer.To("10m"),
				Input:            pointer.To(true),
				NoColor:          pointer.To(true),
				Out:              pointer.To("outfile"),
			},
			exp: []string{
				"-compact-warnings",
				"-destroy",
				"-detailed-exitcode",
				"-no-color",
				"-input=true",
				"-lock=true",
				"-lock-timeout=10m",
				"-out=outfile",
			},
		},
		{
			name: "falsey",
			opts: &PlanOptions{
				CompactWarnings:  pointer.To(false),
				Destroy:          pointer.To(false),
				DetailedExitcode: pointer.To(false),
				Lock:             pointer.To(false),
				LockTimeout:      pointer.To("10m"),
				Input:            pointer.To(false),
				NoColor:          pointer.To(false),
				Out:              pointer.To("outfile"),
			},
			exp: []string{
				"-input=false",
				"-lock=false",
				"-lock-timeout=10m",
				"-out=outfile",
			},
		},
		{
			name: "empty",
			opts: &PlanOptions{},
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

			args := planArgsFromOptions(tc.opts)
			if diff := cmp.Diff(args, tc.exp); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
