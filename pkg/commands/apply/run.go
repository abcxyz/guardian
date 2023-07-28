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

// Package apply provides the Terraform apply functionality for Guardian.
package apply

import (
	"context"

	"github.com/abcxyz/pkg/cli"
)

// RunCommand is a subcommand of apply and implements the cli.Command interface.
// It performs terraform apply on the given working directory.
type RunCommand struct {
	cli.BaseCommand
}

// Desc provides a short, one-line description of the command.
func (c *RunCommand) Desc() string {
	return "Run the Terraform apply for a directory"
}

// Help is the long-form help output to include usage instructions and flag
// information.
func (c *RunCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

	Run the Terraform apply for a directory.
`
}

func (c *RunCommand) Run(ctx context.Context, args []string) error {
	return nil
}
