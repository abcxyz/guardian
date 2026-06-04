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

package workflows

import (
	"testing"

	"github.com/abcxyz/pkg/testutil"
)

func TestReportCommandFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		flagType         string
		flagEntrypoints  string
		flagArtifactsDir string
		err              string
	}{
		{
			name:            "invalid_type",
			flagType:        "invalid",
			flagEntrypoints: "[]",
			err:             "missing or invalid flag: type must be 'plan' or 'apply'",
		},
		{
			name:            "missing_entrypoints",
			flagType:        "apply",
			flagEntrypoints: "",
			err:             "missing flag: entrypoints is required",
		},
		{
			name:             "missing_artifacts_dir_for_plan",
			flagType:         "plan",
			flagEntrypoints:  "[]",
			flagArtifactsDir: "",
			err:              "missing flag: artifacts-dir is required when type is 'plan'",
		},
		{
			name:             "valid_plan",
			flagType:         "plan",
			flagEntrypoints:  "[]",
			flagArtifactsDir: "./artifacts",
		},
		{
			name:            "valid_apply",
			flagType:        "apply",
			flagEntrypoints: "[]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &ReportCommand{}
			f := c.Flags()

			var args []string
			if tc.flagType != "" {
				args = append(args, "-type", tc.flagType)
			}
			if tc.flagEntrypoints != "" {
				args = append(args, "-entrypoints", tc.flagEntrypoints)
			}
			if tc.flagArtifactsDir != "" {
				args = append(args, "-artifacts-dir", tc.flagArtifactsDir)
			}

			err := f.Parse(args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
		})
	}
}
