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
				CompactWarnings:  util.Ptr(true),
				DetailedExitcode: util.Ptr(true),
				Lock:             util.Ptr(true),
				LockTimeout:      util.Ptr("10m"),
				Input:            util.Ptr(true),
				NoColor:          util.Ptr(true),
				Out:              util.Ptr("outfile"),
			},
			exp: []string{
				"plan",
				"-compact-warnings",
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
				CompactWarnings:  util.Ptr(false),
				DetailedExitcode: util.Ptr(false),
				Lock:             util.Ptr(false),
				LockTimeout:      util.Ptr("10m"),
				Input:            util.Ptr(false),
				NoColor:          util.Ptr(false),
				Out:              util.Ptr("outfile"),
			},
			exp: []string{
				"plan",
				"-input=false",
				"-lock=false",
				"-lock-timeout=10m",
				"-out=outfile",
			},
		},
		{
			name: "empty",
			opts: &PlanOptions{},
			exp:  []string{"plan"},
		},
		{
			name: "nil",
			opts: nil,
			exp:  []string{"plan"},
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
