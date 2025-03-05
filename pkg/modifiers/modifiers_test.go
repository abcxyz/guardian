// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modifiers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseMetaValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		contents string
		exp      MetaValues
	}{
		{
			name:     "success",
			contents: "GUARDIAN_DIR=test-plan",
			exp: MetaValues{
				"GUARDIAN_DIR": []string{"test-plan"},
			},
		},
		{
			name: "success_multiple",
			contents: `GUARDIAN_DIR=test-plan1
GUARDIAN_DIR=test-plan2
GUARDIAN_DIR=test-plan3`,
			exp: MetaValues{
				"GUARDIAN_DIR": []string{
					"test-plan1",
					"test-plan2",
					"test-plan3",
				},
			},
		},
		{
			name: "success_mixed",
			contents: `this is a body of text
GUARDIAN_VALUE=test-value1
that contains
GUARDIAN_VALUE=test-value2
no kv pairs
`,
			exp: MetaValues{
				"GUARDIAN_VALUE": []string{"test-value1", "test-value2"},
			},
		},
		{
			name: "no_modifiers",
			contents: `this is a body of text
that contains
no kv pairs
`,
			exp: MetaValues{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := ParseBodyMetaValues(t.Context(), tc.contents)
			if diff := cmp.Diff(m, tc.exp); diff != "" {
				t.Errorf("response not as expected; (-got,+want): %s", diff)
			}
		})
	}
}
