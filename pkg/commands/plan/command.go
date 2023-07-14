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

package plan

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/sethvargo/go-githubactions"
)

var _ cli.Command = (*PlanCommand)(nil)

type PlanCommand struct {
	cli.BaseCommand

	cfg *Config

	flagGitHubToken       string
	flagConcurrency       int64
	flagWorkingDirectory  string
	flagBucketName        string
	flagProtectLockfile   bool
	flagLockTimeout       time.Duration
	flagMaxRetries        uint64
	flagInitialRetryDelay time.Duration
	flagMaxRetryDelay     time.Duration

	planFilename string

	actions         *githubactions.Action
	gitClient       git.Git
	githubClient    github.GitHub
	storageClient   storage.Storage
	terraformClient terraform.Terraform

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option
}

func (c *PlanCommand) Desc() string {
	return `Run the Terraform plan for a directory.`
}

func (c *PlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Run the Terraform plan for a directory.
`
}

func (c *PlanCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	f := set.NewSection("Command options")

	f.StringVar(&cli.StringVar{
		Name:   "github-token",
		EnvVar: "GITHUB_TOKEN",
		Target: &c.flagGitHubToken,
		Usage:  "The GitHub access token to make GitHub API calls.",
	})

	f.Int64Var(&cli.Int64Var{
		Name:    "concurrency",
		EnvVar:  "INPUTS_CONCURRENCY",
		Target:  &c.flagConcurrency,
		Default: 0, // 0 will default to the number of cores on the host machine
		Usage:   "The number of concurrent worker threads.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "working-directory",
		Target:  &c.flagWorkingDirectory,
		Example: "terraform",
		EnvVar:  "INPUTS_WORKING_DIRECTORY",
		Usage:   "The working directory for Guardian to execute commands in.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "bucket-name",
		Target:  &c.flagBucketName,
		Example: "my-guardian-state-bucket",
		EnvVar:  "INPUTS_BUCKET_NAME",
		Usage:   "The Google Cloud Storage bucket name to store Guardian plan files.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "protect-lockfile",
		EnvVar:  "INPUTS_PROTECT_LOCKFILE",
		Target:  &c.flagProtectLockfile,
		Default: true,
		Example: "true",
		Usage:   "Prevent modification of the Terraform lockfile.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "lock-timeout",
		EnvVar:  "INPUTS_LOCK_TIMEOUT",
		Target:  &c.flagLockTimeout,
		Default: 10 * time.Minute,
		Example: "10m",
		Usage:   "The duration Terraform should wait to obtain a lock when running commands that modify state.",
	})

	f.Uint64Var(&cli.Uint64Var{
		Name:    "max-retries",
		EnvVar:  "INPUTS_MAX_RETRIES",
		Target:  &c.flagMaxRetries,
		Default: uint64(3),
		Example: "3",
		Usage:   "The maxinum number of attempts to retry any failures.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "initial-retry-delay",
		EnvVar:  "INPUTS_INITIAL_RETRY_DELAY",
		Target:  &c.flagInitialRetryDelay,
		Default: 2 * time.Second,
		Example: "2s",
		Usage:   "The initial duration to wait before retrying any failures.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "max-retry-delay",
		EnvVar:  "INPUTS_MAX_RETRY_DELAY",
		Target:  &c.flagMaxRetryDelay,
		Default: 1 * time.Minute,
		Example: "1m",
		Usage:   "The maximum duration to wait before retrying any failures.",
	})

	return set
}

func (c *PlanCommand) Run(ctx context.Context, args []string) error {
	logger := logging.FromContext(ctx)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	if c.flagWorkingDirectory == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		c.flagWorkingDirectory = cwd
	}

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	c.actions = githubactions.New(githubactions.WithWriter(c.Stdout()))
	actionsCtx, err := c.actions.Context()
	if err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}

	c.cfg = &Config{}
	if err := c.cfg.MapGitHubContext(actionsCtx); err != nil {
		return fmt.Errorf("failed to load github context: %w", err)
	}
	logger.Debugw("loaded configuration", "config", c.cfg)

	c.gitClient = git.NewGitClient(c.flagWorkingDirectory)
	c.githubClient = github.NewClient(ctx, c.flagGitHubToken, github.WithMaxRetries(c.flagMaxRetries), github.WithMaxRetryDelay(c.flagMaxRetryDelay))
	c.terraformClient = terraform.NewTerraformClient(c.flagWorkingDirectory)

	sc, err := storage.NewGoogleCloudStorage(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create google cloud storage client: %w", err)
	}
	c.storageClient = sc

	return c.Process(ctx)
}
