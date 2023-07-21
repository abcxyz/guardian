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

package iam

import (
	"testing"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/cloudresourcemanager/v3"
)

func Test_removeFromPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		iam    *assetinventory.AssetIAM
		policy *cloudresourcemanager.Policy
		want   *cloudresourcemanager.Policy
	}{
		{
			name: "success_no_condition",
			iam: &assetinventory.AssetIAM{
				ResourceID:   "123",
				ResourceType: assetinventory.Folder,
				Role:         "roles/editor",
				Member:       "user:12312@google.com",
			},
			policy: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
				},
				Version: 3,
			},
			want: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
				},
				Version: 3,
			},
		},
		{
			name: "success_condition",
			iam: &assetinventory.AssetIAM{
				ResourceID:   "123",
				ResourceType: assetinventory.Folder,
				Role:         "roles/editor",
				Member:       "user:12312@google.com",
				Condition:    &assetinventory.IAMCondition{Title: "my-condition", Expression: "request.time > 0", Description: "my description"},
			},
			policy: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
					{
						Members:   []string{"user:12312@google.com"},
						Role:      "roles/editor",
						Condition: &cloudresourcemanager.Expr{Title: "my-condition", Expression: "request.time > 0", Description: "my description"},
					},
				},
				Version: 3,
			},
			want: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
				},
				Version: 3,
			},
		},
		{
			name: "success_no_match_condition",
			iam: &assetinventory.AssetIAM{
				ResourceID:   "123",
				ResourceType: assetinventory.Folder,
				Role:         "roles/editor",
				Member:       "user:12312@google.com",
				Condition:    &assetinventory.IAMCondition{Title: "my-condition", Expression: "request.time > 0", Description: "my description"},
			},
			policy: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
				},
				Version: 3,
			},
			want: &cloudresourcemanager.Policy{
				Etag: "asdasda123",
				Bindings: []*cloudresourcemanager.Binding{
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/viewer",
					},
					{
						Members: []string{"user:12312@google.com", "serviceAccount:123123@prod.google.com"},
						Role:    "roles/editor",
					},
				},
				Version: 3,
			},
		},
	}
	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Run test.
			got := removeFromPolicy(tc.policy, tc.iam)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Process(%+v) got diff (-want, +got): %v", tc.name, diff)
			}
		})
	}
}
