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

// Package Terraform defines an SDK for running Terraform commands using
// the Terraform CLI binary.
package terraform

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/abcxyz/guardian/pkg/child"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"golang.org/x/exp/maps"
)

var (
	tildeChanged = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			"([~])") // tilde represents changes and needs switched to exclamation for git diff

	swapLeadingWhitespace = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			`((\-(\/\+)*)|(\+(\/\-)*)|(!))`) // match characters to swap whitespace for git diff (+, +/-, -, -/+, !)
)

var _ Terraform = (*TerraformClient)(nil)

// Terraform is the interface for working with the Terraform CLI.
type Terraform interface {
	// Init runs the terraform init command.
	Init(context.Context, ...string) (*TerraformResponse, error)

	// Validate runs the terraform validate command.
	Validate(context.Context, ...string) (*TerraformResponse, error)

	// Plan runs the terraform plan command.
	Plan(context.Context, string, ...string) (*TerraformResponse, error)

	// Apply runs the terraform apply command.
	Apply(context.Context, string, ...string) (*TerraformResponse, error)

	// Show runs the terraform show command.
	Show(context.Context, string, ...string) (*TerraformResponse, error)
}

// TerraformClient implements the Terraform interface.
type TerraformClient struct {
	workingDir string
}

// TerraformResponse is the response from running a terraform command.
type TerraformResponse struct {
	Stdout   io.Reader
	Stderr   io.Reader
	ExitCode int
}

// NewTerraformClient creates a new Terraform client.
func NewTerraformClient(workingDir string) *TerraformClient {
	return &TerraformClient{
		workingDir: workingDir,
	}
}

// Init runs the terraform init command.
func (t *TerraformClient) Init(ctx context.Context, args ...string) (*TerraformResponse, error) {
	childArgs := []string{"init"}
	childArgs = append(childArgs, args...)

	result, err := child.Run(ctx, &child.RunConfig{
		WorkingDir: t.workingDir,
		Command:    "terraform",
		Args:       childArgs,
	})

	return &TerraformResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, err //nolint:wrapcheck
}

// Validate runs the terraform validate command.
func (t *TerraformClient) Validate(ctx context.Context, args ...string) (*TerraformResponse, error) {
	childArgs := []string{"validate"}
	childArgs = append(childArgs, args...)

	result, err := child.Run(ctx, &child.RunConfig{
		WorkingDir: t.workingDir,
		Command:    "terraform",
		Args:       childArgs,
	})

	return &TerraformResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, err //nolint:wrapcheck
}

// Show runs the Terraform show command.
func (t *TerraformClient) Show(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	childArgs := []string{"show"}
	childArgs = append(childArgs, args...)

	if file != "" {
		childArgs = append(childArgs, file)
	}

	result, err := child.Run(ctx, &child.RunConfig{
		WorkingDir: t.workingDir,
		Command:    "terraform",
		Args:       childArgs,
	})

	return &TerraformResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, err //nolint:wrapcheck
}

// Plan runs the Terraform plan command.
func (t *TerraformClient) Plan(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	childArgs := []string{"plan"}
	childArgs = append(childArgs, args...)

	if file != "" {
		childArgs = append(childArgs, fmt.Sprintf("-out=%s", file))
	}

	result, err := child.Run(ctx, &child.RunConfig{
		WorkingDir: t.workingDir,
		Command:    "terraform",
		Args:       childArgs,
	})

	return &TerraformResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, err //nolint:wrapcheck
}

// Apply runs the Terraform apply command.
func (t *TerraformClient) Apply(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	childArgs := []string{"apply"}
	childArgs = append(childArgs, args...)

	if file != "" {
		childArgs = append(childArgs, file)
	}

	result, err := child.Run(ctx, &child.RunConfig{
		WorkingDir: t.workingDir,
		Command:    "terraform",
		Args:       childArgs,
	})

	return &TerraformResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, err //nolint:wrapcheck
}

// GetEntrypointDirectories gets all the directories that have Terraform config
// files containing a backend block to be used as an entrypoint module.
func GetEntrypointDirectories(rootDir string) ([]string, error) {
	matches := make(map[string]struct{})
	if err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", path, err)
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".tf" {
			return nil
		}

		hasBackend, _, err := hasBackendConfig(path)
		if err != nil {
			return fmt.Errorf("failed to find terraform backend config: %w", err)
		}

		if !hasBackend {
			return nil
		}

		matches[filepath.Dir(path)] = struct{}{}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	dirs := maps.Keys(matches)

	sort.Strings(dirs)

	return dirs, nil
}

// hasBackendConfig tests a Terraform config file for the existence of a backend block.
func hasBackendConfig(path string) (bool, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	parser := hclparse.NewParser()
	file, d := parser.ParseHCLFile(path)
	diags = append(diags, d...)

	if d.HasErrors() {
		for _, diag := range d {
			if diag.Summary == "Failed to read file" {
				return false, d, fmt.Errorf("failed to read file: %s", path)
			}
		}
	}

	rootBlocks, _, d := file.Body.PartialContent(RootSchema)
	diags = append(diags, d...)

	for _, terraformBlock := range rootBlocks.Blocks.OfType("terraform") {
		content, _, d := terraformBlock.Body.PartialContent(TerraformSchema)
		diags = append(diags, d...)

		innerBlocks := content.Blocks.OfType("backend")
		if len(innerBlocks) > 0 {
			return true, diags, nil
		}
	}

	return false, diags, nil
}

// FormatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func FormatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
