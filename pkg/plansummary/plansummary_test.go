// Copyright 2026 The Authors (see AUTHORS file)
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

package plansummary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/testutil"
)

func TestFromPlanJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		entrypoint string
		planJSON   string
		want       *Summary
		err        string
	}{
		{
			name:       "counts_actions",
			entrypoint: "terraform/github/abseil",
			planJSON: `{
				"resource_changes": [
					{"address": "res1", "change": {"actions": ["create"]}},
					{"address": "res2", "change": {"actions": ["update"]}},
					{"address": "res3", "change": {"actions": ["delete"]}},
					{"address": "res4", "change": {"actions": ["delete", "create"]}},
					{"address": "res5", "change": {"actions": ["no-op"]}},
					{"address": "res6", "change": {"actions": ["read"]}}
				]
			}`,
			want: &Summary{Entrypoint: "terraform/github/abseil", Added: 2, Changed: 1, Deleted: 2},
		},
		{
			name:       "no_changes",
			entrypoint: "./terraform/github/abseil/",
			planJSON:   `{"format_version": "1.2"}`,
			want:       &Summary{Entrypoint: "terraform/github/abseil"},
		},
		{
			name:       "invalid_json",
			entrypoint: "terraform/github/abseil",
			planJSON:   `not json`,
			err:        "failed to parse plan JSON",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := FromPlanJSON(tc.entrypoint, []byte(tc.planJSON))
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("summary mismatch (-got,+want):\n%s", diff)
			}
		})
	}
}

func TestFromPlanJSON_ExcludesPlanContent(t *testing.T) {
	t.Parallel()

	planJSON := `{
		"variables": {"token": {"value": "super-secret-variable"}},
		"prior_state": {"values": {"root_module": {"resources": [{"values": {"password": "super-secret-state"}}]}}},
		"resource_changes": [
			{"address": "google_secret_manager_secret_version.super_secret_address", "change": {"actions": ["create"], "after": {"secret_data": "super-secret-after"}}}
		]
	}`

	s, err := FromPlanJSON("terraform/github/abseil", []byte(planJSON))
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(t.TempDir(), Filename)
	if err := Write(p, s); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "super-secret") || strings.Contains(string(b), "super_secret") {
		t.Errorf("plan summary contains plan content: %s", b)
	}
}

func TestWriteRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	want := &Summary{Entrypoint: "terraform/github/abseil", Added: 1, Changed: 2, Deleted: 3}
	p := filepath.Join(dir, Filename)
	if err := Write(p, want); err != nil {
		t.Fatal(err)
	}

	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("summary mismatch (-got,+want):\n%s", diff)
	}
}

func TestRead_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		err     string
	}{
		{
			name:    "invalid_json",
			content: `not json`,
			err:     "failed to parse plan summary",
		},
		{
			name:    "missing_entrypoint",
			content: `{"added": 1}`,
			err:     "is missing entrypoint",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := filepath.Join(t.TempDir(), Filename)
			if err := os.WriteFile(p, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := Read(p)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
		})
	}
}
