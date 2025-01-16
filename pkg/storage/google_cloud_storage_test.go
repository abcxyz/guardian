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

package storage

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMakeCreateConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		length int
		opts   []CreateOption
		exp    *createConfig
	}{
		{
			name:   "defaults_small_file",
			length: 1 * MiB,
			exp: &createConfig{
				chunkSize:          1*MiB + 256,
				cacheMaxAgeSeconds: 86400,
				cacheControl:       "public, max-age=86400",
				contentType:        "",
				metadata:           nil,
			},
		},
		{
			name:   "defaults_large_file",
			length: 30 * MiB,
			exp: &createConfig{
				chunkSize:          16 * MiB,
				cacheMaxAgeSeconds: 86400,
				cacheControl:       "public, max-age=86400",
				contentType:        "",
				metadata:           nil,
			},
		},
		{
			name:   "overwrites_with_opts",
			length: 1 * MiB,
			opts: []CreateOption{
				WithChunkSize(1000),
				WithCacheMaxAgeSeconds(1000),
				WithContentType("application/json"),
				WithMetadata(map[string]string{
					"key": "value",
				}),
			},
			exp: &createConfig{
				chunkSize:          1000,
				cacheMaxAgeSeconds: 1000,
				cacheControl:       "public, max-age=1000",
				contentType:        "application/json",
				metadata: map[string]string{
					"key": "value",
				},
			},
		},
		{
			name:   "prevents_caching",
			length: 100 * MiB,
			opts: []CreateOption{
				WithCacheMaxAgeSeconds(0),
			},
			exp: &createConfig{
				chunkSize:          16 * MiB,
				cacheMaxAgeSeconds: 0,
				cacheControl:       "no-cache, max-age=0",
				contentType:        "",
				metadata:           nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := makeCreateConfig(tc.length, tc.opts)
			if diff := cmp.Diff(cfg, tc.exp, cmp.Options{cmp.AllowUnexported(createConfig{})}); diff != "" {
				t.Error(diff)
			}
		})
	}
}
