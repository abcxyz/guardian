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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/posener/complete/v2"

	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/platform"
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

	ownerReadWritePerms = 0o600
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
	storagePrefix string

	platformConfig platform.Config

	flags.CommonFlags

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

	c.platformConfig.RegisterFlags(set)
	c.CommonFlags.Register(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "storage",
		Target:  &c.flagStorage,
		Example: "gcs://my-guardian-state-bucket",
		Usage:   fmt.Sprintf("The storage strategy for saving Guardian plan files. Defaults to current working directory. Valid values are %q.", storage.SortedStorageTypes),
		Predict: complete.PredictFunc(func(prefix string) []string {
			return storage.SortedStorageTypes
		}),
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
		c.flagStorage = path.Join("file://", cwd)
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

	storagePrefix, err := c.platformConfig.StoragePrefix()
	if err != nil {
		return fmt.Errorf("failed to parse storage flag: %w", err)
	}
	c.storagePrefix = storagePrefix

	sc, err := storage.Parse(ctx, c.flagStorage)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	c.storageClient = sc

	rc, err := reporter.NewReporter(ctx, c.platformConfig.Reporter, &reporter.Config{GitHub: c.platformConfig.GitHub})
	if err != nil {
		return fmt.Errorf("failed to create reporter client: %w", err)
	}
	c.reporterClient = rc

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian plan run process.
func (c *PlanCommand) Process(ctx context.Context) error {
	var merr error

	util.Headerf(c.Stdout(), ("Starting Guardian Plan"))

	if c.planFilename == "" {
		c.planFilename = "tfplan.binary"
	}

	rp := &reporter.StatusParams{
		Operation: "plan",
		IsDestroy: c.flagDestroy,
		Dir:       c.childPath,
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

	if err := c.reporterClient.Status(ctx, status, rp); err != nil {
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

	util.Headerf(c.Stdout(), "Writing Plan to Local JSON File")
	_, err = c.terraformClient.Show(ctx, &stdout, multiStderr, &terraform.ShowOptions{
		File:    pointer.To(c.planFilename),
		NoColor: pointer.To(true),
		JSON:    pointer.To(true),
	})
	if err != nil {
		return &RunResult{
			commentDetails: stderr.String(),
			hasChanges:     hasChanges,
		}, fmt.Errorf("failed to terraform show: %w", err)
	}

	planBasename := strings.TrimSuffix(c.planFilename, path.Ext(c.planFilename))
	jsonFilepath := path.Join(c.childPath, planBasename+".json")

	var data any
	if err := json.Unmarshal([]byte(stdout.String()), &data); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to unmarshal json plan: %w", err)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to encode json: %w", err)
	}

	if err := os.WriteFile(jsonFilepath, buf.Bytes(), ownerReadWritePerms); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to write plan to json file: %w", err)
	}
	c.Outf("Plan JSON file path: %s", jsonFilepath)

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

	util.Headerf(c.Stdout(), "Saving Plan File")

	if err := c.saveGuardianPlan(ctx, planFileLocalPath, planData, planExitCode); err != nil {
		return &RunResult{hasChanges: hasChanges}, fmt.Errorf("failed to upload plan data: %w", err)
	}

	stderr.Reset()

	return &RunResult{
		commentDetails: stdout.String(),
		hasChanges:     hasChanges,
	}, nil
}

// saveGuardianPlan uploads the Guardian plan binary to the configured Guardian storage client.
func (c *PlanCommand) saveGuardianPlan(ctx context.Context, p string, data []byte, exitCode int) error {
	metadata := make(map[string]string)
	metadata[MetaKeyExitCode] = strconv.Itoa(exitCode)

	metadata[MetaKeyOperation] = OperationPlan
	if c.flagDestroy {
		metadata[MetaKeyOperation] = OperationDestroy
	}

	objectPath := path.Join(c.storagePrefix, p)

	c.Outf("Plan file path: %s %s", c.storageClient.Parent(), objectPath)

	if err := c.storageClient.CreateObject(ctx, objectPath, data,
		storage.WithContentType("application/octet-stream"),
		storage.WithMetadata(metadata),
		storage.WithAllowOverwrite(true),
	); err != nil {
		return fmt.Errorf("failed to save plan file: %w", err)
	}

	return nil
}
