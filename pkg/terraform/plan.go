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

// PlanOptions are the set of options for running a terraform plan.
type PlanOptions struct {
	CompactWarnings  *bool
	DetailedExitcode *bool
	NoColor          *bool
	Input            *bool
	Lock             *bool
	LockTimeout      *string
	Out              *string
}

// planArgsFromOptions generated the terrafrom plan arguments from the provided options.
func planArgsFromOptions(opts *PlanOptions) []string {
	args := make([]string, 0, 7) // 7 potential args to be added

	if opts == nil {
		return args
	}

	if util.BoolVal(opts.CompactWarnings) {
		args = append(args, "-compact-warnings")
	}

	if util.BoolVal(opts.DetailedExitcode) {
		args = append(args, "-detailed-exitcode")
	}

	if util.BoolVal(opts.NoColor) {
		args = append(args, "-no-color")
	}

	if opts.Input != nil {
		args = append(args, fmt.Sprintf("-input=%t", *opts.Input))
	}

	if opts.Lock != nil {
		args = append(args, fmt.Sprintf("-lock=%t", *opts.Lock))
	}

	if opts.LockTimeout != nil {
		args = append(args, fmt.Sprintf("-lock-timeout=%s", *opts.LockTimeout))
	}

	if opts.Out != nil {
		args = append(args, fmt.Sprintf("-out=%s", *opts.Out))
	}

	return args
}

// Plan runs the Terraform plan command.
func (t *TerraformClient) Plan(ctx context.Context, stdout, stderr io.Writer, opts *PlanOptions) (int, error) {
	return t.Run(ctx, stdout, stderr, "plan", planArgsFromOptions(opts)...)
}
