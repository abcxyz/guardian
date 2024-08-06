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

	"github.com/abcxyz/pkg/pointer"
)

// ShowOptions are the set of options for running a terraform show.
type ShowOptions struct {
	File    *string
	NoColor *bool
	JSON    *bool
}

// showArgsFromOptions generated the terrafrom show arguments from the provided options.
func showArgsFromOptions(opts *ShowOptions) []string {
	args := make([]string, 0, 3) // 3 potential args to be added

	if opts == nil {
		return args
	}

	if pointer.Deref(opts.NoColor) {
		args = append(args, "-no-color")
	}

	if pointer.Deref(opts.JSON) {
		args = append(args, "-json")
	}

	if opts.File != nil {
		args = append(args, *opts.File)
	}

	return args
}

// Show runs the Terraform show command.
func (t *TerraformClient) Show(ctx context.Context, stdout, stderr io.Writer, opts *ShowOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "show", showArgsFromOptions(opts)...)
}
