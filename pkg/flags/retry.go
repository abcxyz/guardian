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

package flags

import (
	"time"

	"github.com/abcxyz/pkg/cli"
)

// RetryFlags represent the shared retry flags among all commands.
// Embed this struct into any commands that need retries.
type RetryFlags struct {
	FlagMaxRetries        uint64
	FlagInitialRetryDelay time.Duration
	FlagMaxRetryDelay     time.Duration
}

func (r *RetryFlags) AddFlags(set *cli.FlagSet) {
	f := set.NewSection("Retry options")

	f.Uint64Var(&cli.Uint64Var{
		Name:    "max-retries",
		Target:  &r.FlagMaxRetries,
		Default: uint64(3),
		Example: "3",
		Usage:   "The maxinum number of attempts to retry any failures.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "initial-retry-delay",
		Target:  &r.FlagInitialRetryDelay,
		Default: 2 * time.Second,
		Example: "2s",
		Usage:   "The initial duration to wait before retrying any failures.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "max-retry-delay",
		Target:  &r.FlagMaxRetryDelay,
		Default: 1 * time.Minute,
		Example: "1m",
		Usage:   "The maximum duration to wait before retrying any failures.",
	})
}
