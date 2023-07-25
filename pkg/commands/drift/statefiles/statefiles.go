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

// Package statefiles provide the drift detection on Terraform statefile functionality for Guardian.
package statefiles

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
	"golang.org/x/exp/maps"
)

var _ cli.Command = (*DriftStatefilesCommand)(nil)

type DriftStatefilesCommand struct {
	cli.BaseCommand

	directory string

	flags.GitHubFlags
	flags.RetryFlags

	flagGCSBucket string

	githubClient    github.GitHub
	terraformParser parser.Terraform
}

func (c *DriftStatefilesCommand) Desc() string {
	return `Run the drift detection algorithm on all terraform statefiles in a directory`
}

func (c *DriftStatefilesCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

  Run the drift detection for terraform statefiles in a directory.
`
}

func (c *DriftStatefilesCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.AddFlags(set)
	c.RetryFlags.AddFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "gcsBucket",
		Target:  &c.flagGCSBucket,
		Example: "my-gcs-bucket",
		Usage:   "The gcs bucket to compare against local backend configurations. If none is provided then we will parse it from the backend config.",
	})

	return set
}

func (c *DriftStatefilesCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()

	if len(parsedArgs) != 1 {
		return flag.ErrHelp
	}

	dirAbs, err := util.PathEvalAbs(parsedArgs[0])
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	// TODO: Create github issue.
	c.githubClient = github.NewClient(
		ctx,
		c.GitHubFlags.FlagGitHubToken,
		github.WithRetryInitialDelay(c.RetryFlags.FlagRetryInitialDelay),
		github.WithRetryMaxAttempts(c.RetryFlags.FlagRetryMaxAttempts),
		github.WithRetryMaxDelay(c.RetryFlags.FlagRetryMaxDelay),
	)
	c.terraformParser, err = parser.NewTerraformParser(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create terraform parser: %w", err)
	}

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *DriftStatefilesCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		Named("drift.statefiles").
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner)

	logger.Debug("starting Guardian drift statefiles")
	logger.Debug("finding entrypoint directories")

	entrypoints, err := terraform.GetEntrypointDirectories(c.directory)
	if err != nil {
		return fmt.Errorf("failed to find terraform directories: %w", err)
	}
	entrypointBackendFiles := make([]string, 0, len(entrypoints))
	for _, e := range entrypoints {
		entrypointBackendFiles = append(entrypointBackendFiles, e.BackendFile)
	}
	logger.Debugw("terraform entrypoint directories", "entrypoint_backend_files", entrypointBackendFiles)

	expectedURIs := make([]string, 0, len(entrypointBackendFiles))
	gcsBuckets := make([]string, 0, len(entrypointBackendFiles))
	var errs []error
	for _, f := range entrypointBackendFiles {
		config, _, err := terraform.ExtractBackendConfig(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse Terraform backend config: %w", err))
			continue
		}
		if config.GCSBucket == nil || *config.GCSBucket == "" {
			errs = append(errs, fmt.Errorf("unsupported backend type for terraform config at %s - only gcs backends are supported", f))
			continue
		}
		gcsBuckets = append(gcsBuckets, *config.GCSBucket)
		expectedURIs = append(expectedURIs, fmt.Sprintf("gs://%s/%s/default.tfstate", *config.GCSBucket, *config.Prefix))
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to determine statefile gcs URIs: %w", errors.Join(errs...))
	}

	gcsBucket := c.flagGCSBucket
	for _, bucket := range gcsBuckets {
		if gcsBucket == "" {
			gcsBucket = bucket
		}
		if bucket != gcsBucket {
			return fmt.Errorf("found multiple definitions for gcs buckets - expected a single gcs bucket; from configs: %v; expected %s", gcsBuckets, gcsBucket)
		}
	}

	if gcsBucket == "" {
		return fmt.Errorf("unable to determine gcs bucket - please provide the gcsBucket flag or point at a terraform config with gcs backends")
	}

	logger.Debug("finding statefiles in gcs bucket", "gcs_bucket")

	gotURIs, err := c.terraformParser.StateFileURIs(ctx, []string{gcsBucket})
	if err != nil {
		return fmt.Errorf("failed to determine state file URIs for gcs bucket %s: %w", gcsBucket, err)
	}

	statefilesNotInRemote := sets.SubtractMapKeys(Set(expectedURIs), Set(gotURIs))
	statefilesNotInLocal := sets.SubtractMapKeys(Set(gotURIs), Set(expectedURIs))

	changesDetected := len(statefilesNotInRemote) > 0 || len(statefilesNotInLocal) > 0
	m := driftMessage(statefilesNotInRemote, statefilesNotInLocal)
	if changesDetected {
		c.Outf(m)
	}

	return nil
}

func Set(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

func driftMessage(statefilesNotInRemote, statefilesNotInLocal map[string]struct{}) string {
	var msg strings.Builder
	if len(statefilesNotInRemote) > 0 {
		uris := maps.Keys(statefilesNotInRemote)
		msg.WriteString(fmt.Sprintf("Found state locally that are not in remote \n> %s", strings.Join(uris, "\n> ")))
		if len(statefilesNotInLocal) > 0 {
			msg.WriteString("\n\n")
		}
	}
	if len(statefilesNotInLocal) > 0 {
		uris := maps.Keys(statefilesNotInLocal)
		msg.WriteString(fmt.Sprintf("Found statefiles in remote that are not in local \n> %s", strings.Join(uris, "\n> ")))
	}
	return msg.String()
}
