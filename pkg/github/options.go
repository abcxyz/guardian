// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import "time"

// Option is an optional config value for the GitHubClient.
type Option func(*Config) *Config

// WithMaxRetries configures the maximum number of retrys for the GitHub Client.
func WithMaxRetries(maxRetries uint64) Option {
	return func(c *Config) *Config {
		c.maxRetries = maxRetries
		return c
	}
}

// WithInitialRetryDelay configures the initial delay time before sending a retry for the GitHub Client.
func WithInitialRetryDelay(initialRetryDelay time.Duration) Option {
	return func(c *Config) *Config {
		c.initialRetryDelay = initialRetryDelay
		return c
	}
}

// WithMaxRetryDelay configures the maximum delay time before sending a retry for the GitHub Client.
func WithMaxRetryDelay(maxRetryDelay time.Duration) Option {
	return func(c *Config) *Config {
		c.maxRetryDelay = maxRetryDelay
		return c
	}
}
