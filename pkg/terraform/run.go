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

	"github.com/abcxyz/guardian/pkg/child"
)

// Run runs a Terraform command.
func (t *TerraformClient) Run(ctx context.Context, stdout, stderr io.Writer, subcommand string, args ...string) (int, error) {
	runArgs := []string{subcommand}

	if len(args) > 0 {
		runArgs = append(runArgs, args...)
	}

	return child.Run(ctx, &child.RunConfig{ //nolint:wrapcheck
		Stdout:         stdout,
		Stderr:         stderr,
		WorkingDir:     t.workingDir,
		Command:        "terraform",
		Args:           runArgs,
		AllowedEnvKeys: []string{"*"},
		DeniedEnvKeys:  []string{"TF_CLI_CONFIG_FILE"},
	})
}
