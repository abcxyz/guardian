// Copyright 2024 The Authors (see AUTHORS file)
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

// Package actions provide the support for creating cli commands that run in GitHub actions.
package actions

import (
	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/pkg/cli"
)

type GitHubActionCommand struct {
	cli.BaseCommand

	flags.GitHubFlags

	Action *githubactions.Action
}

// WithActionsOutGroup runs a function and ensures it is wrapped in GitHub actions
// grouping syntax. If this is not in an action, output is printed without grouping syntax.
func (c *GitHubActionCommand) WithActionsOutGroup(msg string, fn func() error) error {
	if c.FlagIsGitHubActions {
		c.Action.Group(msg)
		defer c.Action.EndGroup()
	} else {
		c.Outf(msg)
	}
	return fn()
}
