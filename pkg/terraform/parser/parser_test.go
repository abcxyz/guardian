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

package parser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/commands/drift/gcs"
	"github.com/abcxyz/guardian/pkg/iam"
	"github.com/google/go-cmp/cmp"
)

func TestParser_StateFileURIs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		gcsClient  gcs.GCS
		gcsBuckets []string
		want       []string
		wantErr    string
	}{
		{
			name: "success",
			gcsClient: &gcs.MockStorageClient{FilesURIs: []string{
				"gs://my-bucket-123/abcsdasd/12312/default.tfstate",
				"gs://my-bucket-123/abcsdasd/12313/default.tfstate",
			}},
			want: []string{
				"gs://my-bucket-123/abcsdasd/12312/default.tfstate",
				"gs://my-bucket-123/abcsdasd/12313/default.tfstate",
			},
			gcsBuckets: []string{"my-bucket-123"},
		},
		{
			name:       "failure",
			gcsClient:  &gcs.MockStorageClient{FilesErr: "Failed cause 404"},
			gcsBuckets: []string{"my-bucket-123"},
			wantErr:    "Failed cause 404",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := &TerraformParser{gcs: tc.gcsClient}

			got, err := p.StateFileURIs(context.Background(), tc.gcsBuckets)
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("StateFileURIs: failed to get error %s", tc.wantErr)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

const (
	orgID                = "1231231"
	complexStatefileJSON = `{

	}`
)

func TestParser_ProcessStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		terraformStatefileJSON string
		gcsUris                []string
		want                   []*iam.AssetIAM
		wantErr                string
	}{
		{
			name:                   "success",
			terraformStatefileJSON: complexStatefileJSON,
			want:                   []*iam.AssetIAM{},
			gcsUris:                []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
		},
		{
			name:    "failure",
			gcsUris: []string{"gs://my-bucket-123/abcsdasd/12312/default.tfstate"},
			wantErr: "Failed cause 404",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			jsonBytes, err := json.Marshal(tc.terraformStatefileJSON)
			if err != nil {
				t.Errorf("StateFileURIs: failed to marshal json %s", tc.wantErr)
			}

			gcsClient := &gcs.MockStorageClient{DownloadBytes: jsonBytes}
			p := &TerraformParser{gcs: gcsClient, organizationID: orgID}

			got, err := p.StateFileURIs(context.Background(), tc.gcsUris)
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("StateFileURIs: failed to get error %s", tc.wantErr)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
