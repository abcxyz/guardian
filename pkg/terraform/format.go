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

	"github.com/abcxyz/guardian/pkg/util"
)

// FormatOptions are the set of options for running terraform format.
type FormatOptions struct {
	Check     *bool
	Diff      *bool
	List      *bool
	NoColor   *bool
	Recursive *bool
	Write     *bool
}

// formatArgsFromOptions generated the terrafrom format arguments from the provided options.
func formatArgsFromOptions(opts *FormatOptions) []string {
	args := make([]string, 0, 6) // 6 potential args to be added

	if opts == nil {
		return args
	}

	if util.PtrVal(opts.Check) {
		args = append(args, "-check")
	}

	if util.PtrVal(opts.Diff) {
		args = append(args, "-diff")
	}

	if opts.List != nil {
		args = append(args, fmt.Sprintf("-list=%t", *opts.List))
	}

	if util.PtrVal(opts.NoColor) {
		args = append(args, "-no-color")
	}

	if util.PtrVal(opts.Recursive) {
		args = append(args, "-recursive")
	}

	if opts.Write != nil {
		args = append(args, fmt.Sprintf("-write=%t", *opts.Write))
	}

	return args
}

// Format runs the Terraform format command.
func (t *TerraformClient) Format(ctx context.Context, stdout, stderr io.Writer, opts *FormatOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "fmt", formatArgsFromOptions(opts)...)
}
