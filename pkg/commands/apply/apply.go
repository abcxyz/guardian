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
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/posener/complete/v2"

	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/pointer"
)

const (
	ownerReadWritePerms = 0o600
)

var _ cli.Command = (*ApplyCommand)(nil)

// RunResult is the result of a apply operation.
type RunResult struct {
	commentDetails string
}

// ApplyCommand performs terraform apply on the given working directory.
type ApplyCommand struct {
	cli.BaseCommand

	directory         string
	childPath         string
	planFilename      string
	planFileLocalPath string
	storageParent     string
	storagePrefix     string
	isDestroy         bool

	platformConfig *platform.Config

	flags.CommonFlags

	flagReporter             string
	flagStorage              string
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	storageClient   storage.Storage
	terraformClient terraform.Terraform
	reporterClient  reporter.Reporter
}

// Desc provides a short, one-line description of the command.
func (c *ApplyCommand) Desc() string {
	return "Run Terraform apply for a directory"
}

// Help is the long-form help output to include usage instructions and flag
// information.
func (c *ApplyCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Run Terraform apply for a directory.
`
}

func (c *ApplyCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)
	c.CommonFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "reporter",
		Target:  &c.flagReporter,
		Default: reporter.TypeLocal,
		Example: "github",
		Usage:   fmt.Sprintf("The reporting strategy for Guardian status updates. Valid values are %q.", reporter.SortedReporterTypes),
		Predict: complete.PredictFunc(func(prefix string) []string {
			return reporter.SortedReporterTypes
		}),
	})

	f.StringVar(&cli.StringVar{
		Name:    "storage",
		Target:  &c.flagStorage,
		Example: "gcs://my-guardian-state-bucket",
		Usage:   "The storage strategy to store Guardian plan files. Defaults to current working directory.",
	})

	f.BoolVar(&cli.BoolVar{
		Name:    "allow-lockfile-changes",
		Target:  &c.flagAllowLockfileChanges,
		Example: "true",
		Usage:   "Allow modification of the Terraform lockfile.",
	})

	f.DurationVar(&cli.DurationVar{
		Name:    "lock-timeout",
		Target:  &c.flagLockTimeout,
		Default: 10 * time.Minute,
		Example: "10m",
		Usage:   "The duration Terraform should wait to obtain a lock when running commands that modify state.",
	})

	return set
}

func (c *ApplyCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	cwd, err := c.WorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	if c.FlagDir == "" {
		c.FlagDir = cwd
	}

	if c.flagStorage == "" {
		c.flagStorage = path.Join("local://", cwd)
	}

	dirAbs, err := util.PathEvalAbs(c.FlagDir)
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}
	c.directory = dirAbs

	childPath, err := util.ChildPath(cwd, c.directory)
	if err != nil {
		return fmt.Errorf("failed to get child path for current working directory: %w", err)
	}
	c.childPath = childPath

	c.terraformClient = terraform.NewTerraformClient(c.directory)

	// TODO(verbanicm): create plan storage impl of storage
	sc, err := c.resolveStorageClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	c.storageClient = sc

	rc, err := reporter.NewReporter(ctx, c.flagReporter, &reporter.Config{GitHub: *c.platformConfig.GitHub}, c.Stdout())
	if err != nil {
		return fmt.Errorf("failed to create reporter client: %w", err)
	}
	c.reporterClient = rc

	return c.Process(ctx)
}

// resolveStorageClient resolves and generated the storage client based on the storage flag.
func (c *ApplyCommand) resolveStorageClient(ctx context.Context) (storage.Storage, error) {
	u, err := url.Parse(c.flagStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse storage flag url: %w", err)
	}

	t := u.Scheme

	c.storageParent = path.Join(u.Host, u.Path)

	if strings.EqualFold(t, storage.TypeFilesystem) {
		return storage.NewFilesystemStorage(ctx) //nolint:wrapcheck // Want passthrough
	}

	if strings.EqualFold(t, storage.TypeGoogleCloudStorage) {
		sc, err := storage.NewGoogleCloudStorage(ctx)
		if err != nil {
			return nil, err //nolint:wrapcheck // Want passthrough
		}

		if strings.EqualFold(c.platformConfig.Type, platform.TypeGitHub) {
			var merr error
			if c.platformConfig.GitHub.GitHubOwner == "" {
				merr = errors.Join(merr, fmt.Errorf("github owner is required for storage type %s", storage.TypeGoogleCloudStorage))
			}
			if c.platformConfig.GitHub.GitHubRepo == "" {
				merr = errors.Join(merr, fmt.Errorf("github repo is required for storage type %s", storage.TypeGoogleCloudStorage))
			}
			if c.platformConfig.GitHub.GitHubPullRequestNumber <= 0 {
				merr = errors.Join(merr, fmt.Errorf("github pull request number is required for storage type %s", storage.TypeGoogleCloudStorage))
			}

			if merr != nil {
				return nil, merr
			}

			c.storagePrefix = fmt.Sprintf("guardian-plans/%s/%s/%d", c.platformConfig.GitHub.GitHubOwner, c.platformConfig.GitHub.GitHubRepo, c.platformConfig.GitHub.GitHubPullRequestNumber)
		}
		return sc, nil
	}

	return nil, fmt.Errorf("unknown storage type: %s", t)
}

// Process handles the main logic for the Guardian apply run process.
func (c *ApplyCommand) Process(ctx context.Context) (merr error) {
	logger := logging.FromContext(ctx)

	util.Headerf(c.Stdout(), "Starting Guardian Apply")

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	storageObjectPath := path.Join(c.storagePrefix, c.childPath, c.planFilename)
	logger.DebugContext(ctx, "storage object path", "path", storageObjectPath)

	planData, planExitCode, err := c.downloadGuardianPlan(ctx, storageObjectPath)
	if err != nil {
		return fmt.Errorf("failed to download guardian plan file: %w", err)
	}

	// we always want to delete the remote plan file to keep the bucket clean
	defer func() {
		if err := c.deleteGuardianPlan(ctx, storageObjectPath); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to delete plan file: %w", err))
		}
	}()

	// exit code of 0 means success with no diff, skip apply
	if planExitCode == "0" {
		logger.DebugContext(ctx, "plan file has no diff, exiting", "plan_exit_code", planExitCode)
		c.Outf("Guardian plan file has no diff, exiting")
		return
	}

	tempDir, err := os.MkdirTemp("", "guardian-plans-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary plan directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to delete temporary plan directory: %w", err))
		}
	}()

	util.Headerf(c.Stdout(), "Writing plan file to disk")

	planFileLocalPath := filepath.Join(tempDir, c.planFilename)
	if err := os.WriteFile(planFileLocalPath, planData, ownerReadWritePerms); err != nil {
		return fmt.Errorf("failed to write plan file to disk [%s]: %w", planFileLocalPath, err)
	}
	c.planFileLocalPath = planFileLocalPath

	rp := &reporter.Params{
		Operation: "apply",
		IsDestroy: c.isDestroy,
		Dir:       c.directory,
		HasDiff:   true,
	}

	status := reporter.StatusSuccess

	result, err := c.terraformApply(ctx)
	if err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian apply: %w", err))
		status = reporter.StatusFailure
	}

	rp.Details = result.commentDetails

	if err := c.reporterClient.CreateStatus(ctx, status, rp); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to report status: %w", err))
	}

	return merr
}

// terraformApply runs the required Terraform commands for a full run of
// a Guardian apply using the Terraform CLI.
func (c *ApplyCommand) terraformApply(ctx context.Context) (*RunResult, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	util.Headerf(c.Stdout(), "Running Terraform commands")

	lockfileMode := "none"
	if !c.flagAllowLockfileChanges {
		lockfileMode = "readonly"
	}

	util.Headerf(c.Stdout(), "Initializing Terraform")
	if _, err := c.terraformClient.Init(ctx, c.Stdout(), multiStderr, &terraform.InitOptions{
		Input:       pointer.To(false),
		NoColor:     pointer.To(true),
		Lockfile:    pointer.To(lockfileMode),
		LockTimeout: pointer.To(c.flagLockTimeout.String()),
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to initialize: %w", err)
	}

	stderr.Reset()

	util.Headerf(c.Stdout(), "Validating Terraform")
	if _, err := c.terraformClient.Validate(ctx, c.Stdout(), multiStderr, &terraform.ValidateOptions{
		NoColor: pointer.To(true),
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to validate: %w", err)
	}

	stderr.Reset()

	util.Headerf(c.Stdout(), "Applying Terraform")
	if _, err := c.terraformClient.Apply(ctx, multiStdout, multiStderr, &terraform.ApplyOptions{
		File:            pointer.To(c.planFileLocalPath),
		CompactWarnings: pointer.To(true),
		Input:           pointer.To(false),
		NoColor:         pointer.To(true),
		LockTimeout:     pointer.To(c.flagLockTimeout.String()),
	}); err != nil {
		return &RunResult{commentDetails: stderr.String()}, fmt.Errorf("failed to apply: %w", err)
	}

	stderr.Reset()

	util.Headerf(c.Stdout(), "Formatting output")
	githubOutput := terraform.FormatOutputForGitHubDiff(stdout.String())

	return &RunResult{commentDetails: githubOutput}, nil
}

// downloadGuardianPlan downloads the Guardian plan binary from the configured Guardian storage bucket
// and returns the plan data and plan exit code.
func (c *ApplyCommand) downloadGuardianPlan(ctx context.Context, path string) (planData []byte, planExitCode string, outErr error) {
	util.Headerf(c.Stdout(), "Downloading Guardian plan file")

	rc, metadata, err := c.storageClient.GetObject(ctx, c.storageParent, path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download object: %w", err)
	}

	if metadata != nil {
		exitCode, ok := metadata[plan.MetaKeyExitCode]
		if !ok {
			return nil, "", fmt.Errorf("failed to determine plan exit code: %w", err)
		}
		planExitCode = exitCode

		planOperation, ok := metadata[plan.MetaKeyOperation]
		if ok && strings.EqualFold(planOperation, plan.OperationDestroy) {
			c.isDestroy = true
		}
	}

	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			outErr = fmt.Errorf("failed to close get object reader: %w", closeErr)
		}
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read plan data: %w", err)
	}
	planData = data

	return planData, planExitCode, nil
}

// handleDeleteGuardianPlan deletes the Guardian plan binary from the configured Guardian storage bucket.
func (c *ApplyCommand) deleteGuardianPlan(ctx context.Context, path string) error {
	util.Headerf(c.Stdout(), "Deleting Guardian plan file")

	if err := c.storageClient.DeleteObject(ctx, c.storageParent, path); err != nil {
		return fmt.Errorf("failed to delete apply file: %w", err)
	}

	return nil
}
