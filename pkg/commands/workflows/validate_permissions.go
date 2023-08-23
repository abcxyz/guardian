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

// Package provides worklow helper functionality for Guardian.
package workflows

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sort"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/exp/slices"
)

var _ cli.Command = (*ValidatePermissionsCommand)(nil)

type ValidatePermissionsCommand struct {
	cli.BaseCommand

	cfg *ValidatePermissionsConfig

	flags.GitHubFlags
	flags.RetryFlags

	flagAllowedPermissions []string

	actions      *githubactions.Action
	gitHubClient github.GitHub
}

func (c *ValidatePermissionsCommand) Desc() string {
	return `Validate a list of required permissions for the actor running the current GitHub workflow`
}

func (c *ValidatePermissionsCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Validate a list of allowed permissions for the actor running the current GitHub workflow.
`
}

func (c *ValidatePermissionsCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-permissions",
		Target:  &c.flagAllowedPermissions,
		Example: "admin, write",
		Usage:   "The list of allowed permissions to validate against.",
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.GitHubFlags.FlagGitHubOwner == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: github-owner is required"))
		}

		if c.GitHubFlags.FlagGitHubRepo == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: github-repo is required"))
		}

		return merr
	})

	return set
}

func (c *ValidatePermissionsCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.actions.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &ValidatePermissionsConfig{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.DebugContext(ctx, "loaded configuration", "validate_permissions_config", c.cfg)

	c.gitHubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian validate permissions process.
func (c *ValidatePermissionsCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner)

	logger.DebugContext(ctx, "checking required permissions")

	permission, err := c.gitHubClient.RepoUserPermissionLevel(ctx, c.GitHubFlags.FlagGitHubOwner, c.GitHubFlags.FlagGitHubRepo, c.cfg.Actor)
	if err != nil {
		return fmt.Errorf("failed to get repo permissions: %w", err)
	}

	if allowed := slices.Contains(c.flagAllowedPermissions, permission); !allowed {
		sort.Strings(c.flagAllowedPermissions)
		return fmt.Errorf("%s does not have the required permissions to run this command.\n\nRequired permissions are %q", c.cfg.Actor, c.flagAllowedPermissions)
	}

	return nil
}
