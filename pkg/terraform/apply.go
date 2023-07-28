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
)

// ApplyOptions are the set of options for running a terraform apply.
type ApplyOptions struct {
	File            *string
	AutoApprove     *bool
	CompactWarnings *bool
	Lock            *bool
	LockTimeout     *string
	Input           *bool
	NoColor         *bool
}

// applyArgsFromOptions generated the terrafrom apply arguments from the provided options.
func applyArgsFromOptions(opts *ApplyOptions) []string {
	args := make([]string, 0, 7) // 7 potential args to be added

	if opts == nil {
		return args
	}

	if opts.AutoApprove != nil && *opts.AutoApprove {
		args = append(args, "-auto-approve")
	}

	if opts.CompactWarnings != nil && *opts.CompactWarnings {
		args = append(args, "-compact-warnings")
	}

	if opts.Lock != nil {
		args = append(args, fmt.Sprintf("-lock=%t", *opts.Lock))
	}

	if opts.LockTimeout != nil {
		args = append(args, fmt.Sprintf("-lock-timeout=%s", *opts.LockTimeout))
	}

	if opts.Input != nil {
		args = append(args, fmt.Sprintf("-input=%t", *opts.Input))
	}

	if opts.NoColor != nil && *opts.NoColor {
		args = append(args, "-no-color")
	}

	if opts.File != nil {
		args = append(args, *opts.File)
	}

	return args
}

// Apply runs the Terraform apply command.
func (t *TerraformClient) Apply(ctx context.Context, stdout, stderr io.Writer, opts *ApplyOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "apply", applyArgsFromOptions(opts)...)
}
