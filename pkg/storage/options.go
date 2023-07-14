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

// uploadConfig is the set of configuration used when uploading storage objects.
type uploadConfig struct {
	allowOverwrite     bool
	cacheControl       string
	cacheMaxAgeSeconds int
	chunkSize          int
	contentType        string
	metadata           map[string]string
}

// UploadOption is an optional config value for the Google Cloud Storage UploadObject function.
type UploadOption func(*uploadConfig) *uploadConfig

// WithChunkSize configures the chunk size for the object upload. Set this value to 0 to send the
// entire file in a single request.
func WithChunkSize(chunkSize int) UploadOption {
	return func(c *uploadConfig) *uploadConfig {
		c.chunkSize = chunkSize
		return c
	}
}

// WithCacheMaxAgeSeconds configures the cache-control header the object upload. Set this value to 0 to prevent
// caching the file.
func WithCacheMaxAgeSeconds(cacheMaxAgeSeconds int) UploadOption {
	return func(c *uploadConfig) *uploadConfig {
		c.cacheMaxAgeSeconds = cacheMaxAgeSeconds
		return c
	}
}

// WithContentType sets the content type for the object upload.
func WithContentType(contentType string) UploadOption {
	return func(c *uploadConfig) *uploadConfig {
		c.contentType = contentType
		return c
	}
}

// WithAllowOverwrite sets the overwrite flag to allow overwriting the destination object.
func WithAllowOverwrite(allowOverwrite bool) UploadOption {
	return func(c *uploadConfig) *uploadConfig {
		c.allowOverwrite = allowOverwrite
		return c
	}
}

// WithMetadata sets the metadata for the object upload.
func WithMetadata(metadata map[string]string) UploadOption {
	return func(c *uploadConfig) *uploadConfig {
		c.metadata = metadata
		return c
	}
}
