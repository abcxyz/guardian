// Copyright 2023 The Authors (see AUTHORS file)
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

package iamcleanup

import (
	"testing"

	"github.com/abcxyz/pkg/pointer"
	"github.com/abcxyz/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func Test_evaluateIAMConditionExpression(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		expression    string
		want          *bool
		wantErrSubstr string
	}{
		{
			name:       "success_expired",
			expression: "request.time < timestamp('2019-01-01T00:00:00Z')",
			want:       pointer.To(false),
		},
		{
			name:       "success_not_expired",
			expression: "request.time < timestamp('3024-01-01T00:00:00Z')",
			want:       pointer.To(true),
		},
		{
			name:          "failed_to_parse",
			expression:    "request.made_up_time_arg < timestamp('3024-01-01T00:00:00Z')",
			wantErrSubstr: "failed to compile Expression",
		},
		{
			name:          "failed_to_parse",
			expression:    "request.path == '/admin'",
			wantErrSubstr: "unsupported field 'path' in Condition Expression",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Run test.
			got, gotErr := evaluateIAMConditionExpression(t.Context(), tc.expression)
			if diff := testutil.DiffErrString(gotErr, tc.wantErrSubstr); diff != "" {
				t.Errorf("Process(%+v) got unexpected error substring: %v", tc.name, diff)
			}
			// Verify that the ResourceMapping is modified with additional annotations fetched from Asset Inventory.
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Process(%+v) got diff (-want, +got): %v", tc.name, diff)
			}
		})
	}
}
