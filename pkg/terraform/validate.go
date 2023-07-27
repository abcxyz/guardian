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
	"io"
)

// ValidateOptions are the set of options for running a terraform validate.
type ValidateOptions struct {
	NoColor *bool
	JSON    *bool
}

// validateArgsFromOptions generated the terrafrom validate arguments from the provided options.
func validateArgsFromOptions(opts *ValidateOptions) []string {
	args := make([]string, 0, 2) // 2 potential args to be added

	if opts == nil {
		return args
	}

	if opts.NoColor != nil && *opts.NoColor {
		args = append(args, "-no-color")
	}

	if opts.JSON != nil && *opts.JSON {
		args = append(args, "-json")
	}

	return args
}

// Validate runs the terraform validate command.
func (t *TerraformClient) Validate(ctx context.Context, stdout, stderr io.Writer, opts *ValidateOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "validate", validateArgsFromOptions(opts)...)
}
