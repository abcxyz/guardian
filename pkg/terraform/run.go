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
	"slices"

	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/guardian/pkg/child"
)

// overrideEnvVars are the environment variables to inject into the Terraform
// child process, no matter what the user configured. These take precedence
// over all other configurables.
var defaultOverrideEnvVars = []string{
	"GOOGLE_TERRAFORM_USERAGENT_EXTENSION=" + version.UserAgent,
	"TF_APPEND_USER_AGENT=" + version.UserAgent,
}

// Run runs a Terraform command.
func (t *TerraformClient) Run(ctx context.Context, stdout, stderr io.Writer, subcommand string, args ...string) (int, error) {
	runArgs := []string{subcommand}

	if len(args) > 0 {
		runArgs = append(runArgs, args...)
	}

	overrideEnvVars := slices.Concat(nil, t.envVars, defaultOverrideEnvVars)

	return child.Run(ctx, &child.RunConfig{ //nolint:wrapcheck
		Stdout:          stdout,
		Stderr:          stderr,
		WorkingDir:      t.workingDir,
		Command:         "terraform",
		Args:            runArgs,
		OverrideEnvVars: overrideEnvVars,
	})
}
