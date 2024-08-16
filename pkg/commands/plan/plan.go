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

// Package plan provide the Terraform planning functionality for Guardian.
package plan

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/posener/complete/v2"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/pointer"
)

const (
	// plan file metadata key representing the exit code.
	MetaKeyExitCode = "plan_exit_code"

	// plan file metadata key representing the operation (plan, destroy).
	MetaKeyOperation = "operation"

	// plan files metadata operation values.
	OperationPlan    = "plan"
	OperationDestroy = "destroy"
)

var _ cli.Command = (*PlanCommand)(nil)

// RunResult is the result of a plan operation.
type RunResult struct {
	hasChanges     bool
	commentDetails string
}

type PlanCommand struct {
	cli.BaseCommand

	directory     string
	childPath     string
	planFilename  string
	storageParent string
	storagePrefix string

	githubConfig *github.Config

	flags.GlobalFlags
	flags.CommonFlags

	flagReporter             string
	flagDestroy              bool
	flagStorage              string
	flagAllowLockfileChanges bool
	flagLockTimeout          time.Duration

	storageClient   storage.Storage
	terraformClient terraform.Terraform
	reporterClient  reporter.Reporter
}

func (c *PlanCommand) Desc() string {
	return `Run Terraform plan for a directory`
}

func (c *PlanCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Run Terraform plan for a directory.
`
}

func (c *PlanCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.githubConfig.RegisterFlags(set)

	c.GlobalFlags.Register(set)
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
		Name:    "destroy",
		Target:  &c.flagDestroy,
		Example: "true",
		Usage:   "Use the destroy flag to plan changes to destroy all infrastructure.",
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

func (c *PlanCommand) Run(ctx context.Context, args []string) error {
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

	rc, err := reporter.NewReporter(ctx, c.flagReporter, &reporter.Config{GitHub: c.githubConfig}, c.Stdout())
	if err != nil {
		return fmt.Errorf("failed to create reporter client: %w", err)
	}
	c.reporterClient = rc

	return c.Process(ctx)
}

// resolveStorageClient resolves and generated the storage client based on the storage flag.
func (c *PlanCommand) resolveStorageClient(ctx context.Context) (storage.Storage, error) {
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

		if strings.EqualFold(c.GlobalFlags.FlagPlatform, flags.PlatformTypeGitHub) {
			var merr error
			if c.githubConfig.GitHubOwner == "" {
				merr = errors.Join(merr, fmt.Errorf("github owner is required for storage type %s", storage.TypeGoogleCloudStorage))
			}
			if c.githubConfig.GitHubRepo == "" {
				merr = errors.Join(merr, fmt.Errorf("github repo is required for storage type %s", storage.TypeGoogleCloudStorage))
			}
			if c.githubConfig.GitHubPullRequestNumber <= 0 {
				merr = errors.Join(merr, fmt.Errorf("github pull request number is required for storage type %s", storage.TypeGoogleCloudStorage))
			}

			if merr != nil {
				return nil, merr
			}

			c.storagePrefix = fmt.Sprintf("guardian-plans/%s/%s/%d", c.githubConfig.GitHubOwner, c.githubConfig.GitHubRepo, c.githubConfig.GitHubPullRequestNumber)
		}
		return sc, nil
	}

	return nil, fmt.Errorf("unknown storage type: %s", t)
}

// Process handles the main logic for the Guardian plan run process.
func (c *PlanCommand) Process(ctx context.Context) error {
	var merr error

	util.Headerf(c.Stdout(), ("Starting Guardian Plan"))

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	rp := &reporter.Params{
		Operation: "plan",
		IsDestroy: c.flagDestroy,
		Dir:       c.directory,
	}

	status := reporter.StatusNoOperation

	result, err := c.terraformPlan(ctx)
	if err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to run Guardian plan: %w", err))
		status = reporter.StatusFailure
		rp.Details = result.commentDetails
	}

	if result.hasChanges && err == nil {
		status = reporter.StatusSuccess
		rp.Details = result.commentDetails
		rp.HasDiff = true
	}

	if err := c.reporterClient.CreateStatus(ctx, status, rp); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to report status: %w", err))
	}

	return merr
}

// terraformPlan runs the required Terraform commands for a full run of
// a Guardian plan using the Terraform CLI.
func (c *PlanCommand) terraformPlan(ctx context.Context) (*RunResult, error) {
	var stdout, stderr strings.Builder
	multiStdout := io.MultiWriter(c.Stdout(), &stdout)
	multiStderr := io.MultiWriter(c.Stderr(), &stderr)

	util.Headerf(c.Stdout(), "Running Terraform commands")

	util.Headerf(c.Stdout(), "Check Terraform Format")
	if _, err := c.terraformClient.Format(ctx, multiStdout, multiStderr, &terraform.FormatOptions{
		Check:     pointer.To(true),
		Diff:      pointer.To(true),
		Recursive: pointer.To(true),
		NoColor:   pointer.To(true),
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to check formatting: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	lockfileMode := "none"
	if !c.flagAllowLockfileChanges {
		lockfileMode = "readonly"
	}

	util.Headerf(c.Stdout(), "Initializing Terraform")
	if _, err := c.terraformClient.Init(ctx, multiStdout, multiStderr, &terraform.InitOptions{
		Input:       pointer.To(false),
		NoColor:     pointer.To(true),
		Lockfile:    pointer.To(lockfileMode),
		LockTimeout: pointer.To(c.flagLockTimeout.String()),
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to initialize: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	util.Headerf(c.Stdout(), "Validating Terraform")
	if _, err := c.terraformClient.Validate(ctx, multiStdout, multiStderr, &terraform.ValidateOptions{
		NoColor: pointer.To(true),
	}); err != nil {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to validate: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	var hasChanges bool
	var planExitCode int

	util.Headerf(c.Stdout(), "Planning Terraform")
	exitCode, err := c.terraformClient.Plan(ctx, multiStdout, multiStderr, &terraform.PlanOptions{
		Out:              pointer.To(c.planFilename),
		Input:            pointer.To(false),
		NoColor:          pointer.To(true),
		Destroy:          pointer.To(c.flagDestroy),
		DetailedExitcode: pointer.To(true),
		LockTimeout:      pointer.To(c.flagLockTimeout.String()),
	})

	planExitCode = exitCode
	// use the detailed exitcode from terraform to determine if there is a diff
	// 0 - success, no diff  1 - failed   2 - success, diff
	hasChanges = planExitCode == 2

	if err != nil && !hasChanges {
		commentDetails := stderr.String()
		if commentDetails == "" {
			commentDetails = stdout.String()
		}
		return &RunResult{commentDetails: commentDetails}, fmt.Errorf("failed to plan: %w", err)
	}

	stdout.Reset()
	stderr.Reset()

	util.Headerf(c.Stdout(), "Formatting output")
	if _, err := c.terraformClient.Show(ctx, multiStdout, multiStderr, &terraform.ShowOptions{
		File:    pointer.To(c.planFilename),
		NoColor: pointer.To(true),
	}); err != nil {
		return &RunResult{
			commentDetails: stderr.String(),
			hasChanges:     hasChanges,
		}, fmt.Errorf("failed to terraform show: %w", err)
	}

	stderr.Reset()

	planFileLocalPath := path.Join(c.childPath, c.planFilename)

	planData, err := os.ReadFile(planFileLocalPath)
	if err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to read plan binary: %w", err)
	}

	planStoragePath := path.Join(c.storagePrefix, planFileLocalPath)

	util.Headerf(c.Stdout(), "Saving Plan File: %s", path.Join(c.storageParent, planStoragePath))

	if err := c.uploadGuardianPlan(ctx, planStoragePath, planData, planExitCode); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()

	return &RunResult{
		commentDetails: stdout.String(),
		hasChanges:     hasChanges,
	}, nil
}

// uploadGuardianPlan uploads the Guardian plan binary to the configured Guardian storage bucket.
func (c *PlanCommand) uploadGuardianPlan(ctx context.Context, path string, data []byte, exitCode int) error {
	metadata := make(map[string]string)
	metadata[MetaKeyExitCode] = strconv.Itoa(exitCode)

	metadata[MetaKeyOperation] = OperationPlan
	if c.flagDestroy {
		metadata[MetaKeyOperation] = OperationDestroy
	}

	if err := c.storageClient.CreateObject(ctx, c.storageParent, path, data,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to upload plan file: %w", err)
	}

	return nil
}
