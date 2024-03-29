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

	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/pkg/testutil"
)

func TestPlanStatusCommentsConfig_MapGitHubContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		githubContext *githubactions.GitHubContext
		exp           *PlanStatusCommentsConfig
		err           string
	}{
		{
			name: "success",
			githubContext: &githubactions.GitHubContext{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: 1,
			},
			exp: &PlanStatusCommentsConfig{
				ServerURL:  "https://github.com",
				RunID:      1,
				RunAttempt: 1,
			},
		},
		{
			name:          "error",
			githubContext: &githubactions.GitHubContext{},
			exp:           &PlanStatusCommentsConfig{},
			err:           "GITHUB_SERVER_URL is required\nGITHUB_RUN_ID is required\nGITHUB_RUN_ATTEMPT is required",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var c PlanStatusCommentsConfig

			err := c.MapGitHubContext(tc.githubContext)
			if err != nil || tc.err != "" {
				if diff := testutil.DiffErrString(err, tc.err); diff != "" {
					t.Fatal(diff)
				}
			}

			if diff := cmp.Diff(&c, tc.exp); diff != "" {
				t.Errorf("got %#v, want %#v, diff (-got, +want): %v", &c, tc.exp, diff)
			}
		})
	}
}

func TestValidatePermissionsConfig_MapGitHubContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		githubContext *githubactions.GitHubContext
		exp           *ValidatePermissionsConfig
		err           string
	}{
		{
			name: "success",
			githubContext: &githubactions.GitHubContext{
				Actor: "actor",
			},
			exp: &ValidatePermissionsConfig{
				Actor: "actor",
			},
		},
		{
			name:          "error",
			githubContext: &githubactions.GitHubContext{},
			exp:           &ValidatePermissionsConfig{},
			err:           "GITHUB_ACTOR is required",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var c ValidatePermissionsConfig

			err := c.MapGitHubContext(tc.githubContext)
			if err != nil || tc.err != "" {
				if diff := testutil.DiffErrString(err, tc.err); diff != "" {
					t.Fatal(diff)
				}
			}

			if diff := cmp.Diff(&c, tc.exp); diff != "" {
				t.Errorf("got %#v, want %#v, diff (-got, +want): %v", &c, tc.exp, diff)
			}
		})
	}
}
