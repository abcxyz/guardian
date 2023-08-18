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

package apply

import (
	"testing"

	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/sethvargo/go-githubactions"
)

func TestConfig_MapGitHubContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		githubContext *githubactions.GitHubContext
		exp           *Config
		wantErr       string
	}{
		{
			name: "success",
			githubContext: &githubactions.GitHubContext{
				ServerURL:  "https://github.com",
				RunID:      int64(100),
				RunAttempt: int64(1),
			},
			exp: &Config{
				ServerURL:  "https://github.com",
				RunID:      int64(100),
				RunAttempt: int64(1),
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &Config{}

			err := c.MapGitHubContext(tc.githubContext)
			if err != nil || tc.wantErr != "" {
				if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
					t.Fatal(diff)
				}
				return
			}

			if diff := cmp.Diff(c, tc.exp); diff != "" {
				t.Errorf("got %#v, want %#v, diff (-got, +want): %v", c, tc.exp, diff)
			}
		})
	}
}
