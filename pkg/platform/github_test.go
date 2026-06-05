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

package platform

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shurcooL/githubv4"

	gh "github.com/abcxyz/guardian/pkg/github"
)

type roundTripperFunc func(req *http.Request) *http.Response

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestGitHub_GetUserTeamMemberships(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		username     string
		mockResponse []string
		want         []string
		wantErr      string
	}{
		{
			name:     "success_single_page",
			username: "testuser",
			mockResponse: []string{
				`{
					"data": {
						"organization": {
							"teams": {
								"nodes": [
									{
										"name": "team1",
										"members": {
											"nodes": [
												{
													"login": "testuser"
												}
											]
										}
									}
								],
								"pageInfo": {
									"hasNextPage": false,
									"endCursor": "cursor1"
								}
							}
						}
					}
				}`,
			},
			want: []string{"team1"},
		},
		{
			name:     "success_multi_page",
			username: "testuser",
			mockResponse: []string{
				`{
					"data": {
						"organization": {
							"teams": {
								"nodes": [
									{
										"name": "team1",
										"members": {
											"nodes": [
												{
													"login": "testuser"
												}
											]
										}
									}
								],
								"pageInfo": {
									"hasNextPage": true,
									"endCursor": "cursor1"
								}
							}
						}
					}
				}`,
				`{
					"data": {
						"organization": {
							"teams": {
								"nodes": [
									{
										"name": "team2",
										"members": {
											"nodes": [
												{
													"login": "testuser"
												}
											]
										}
									}
								],
								"pageInfo": {
									"hasNextPage": false,
									"endCursor": "cursor2"
								}
							}
						}
					}
				}`,
			},
			want: []string{"team1", "team2"},
		},
		{
			name:     "no_username",
			username: "",
			wantErr:  "username is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reqCount int
			client := githubv4.NewClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) *http.Response {
					if reqCount >= len(tc.mockResponse) {
						return &http.Response{
							StatusCode: http.StatusInternalServerError,
							Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"unexpected request"}]}`)),
						}
					}
					resp := tc.mockResponse[reqCount]
					reqCount++
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(resp)),
					}
				}),
			})

			g := &GitHub{
				cfg: &gh.Config{
					GitHubOwner: "abcxyz",
				},
				graphqlClient: client,
			}

			got, err := g.GetUserTeamMemberships(t.Context(), tc.username)
			if err != nil {
				if tc.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err mismatch got: %v, want to contain: %s", err, tc.wantErr)
				}
			} else if tc.wantErr != "" {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("got mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
