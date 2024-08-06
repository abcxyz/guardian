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

package terraform

import (
	"context"
	"fmt"
	"io"

	"github.com/abcxyz/pkg/pointer"
)

// InitOptions are the set of options for running a terraform init.
type InitOptions struct {
	Backend     *bool
	Input       *bool
	NoColor     *bool
	Lock        *bool
	LockTimeout *string
	Lockfile    *string
}

// initArgsFromOptions generated the terrafrom init arguments from the provided options.
func initArgsFromOptions(opts *InitOptions) []string {
	args := make([]string, 0, 6) // 6 potential args to be added

	if opts == nil {
		return args
	}

	if opts.Backend != nil {
		args = append(args, fmt.Sprintf("-backend=%t", *opts.Backend))
	}

	if opts.Input != nil {
		args = append(args, fmt.Sprintf("-input=%t", *opts.Input))
	}

	if pointer.Deref(opts.NoColor) {
		args = append(args, "-no-color")
	}

	if opts.Lock != nil {
		args = append(args, fmt.Sprintf("-lock=%t", *opts.Lock))
	}

	if opts.LockTimeout != nil {
		args = append(args, fmt.Sprintf("-lock-timeout=%s", *opts.LockTimeout))
	}

	if opts.Lockfile != nil {
		args = append(args, fmt.Sprintf("-lockfile=%s", *opts.Lockfile))
	}

	return args
}

// Init runs the terraform init command.
func (t *TerraformClient) Init(ctx context.Context, stdout, stderr io.Writer, opts *InitOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "init", initArgsFromOptions(opts)...)
}
