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

	"github.com/abcxyz/guardian/pkg/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
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
	Init(context.Context, io.Writer, io.Writer, *InitOptions) (int, error)

	// Validate runs the terraform validate command.
	Validate(context.Context, io.Writer, io.Writer, *ValidateOptions) (int, error)

	// Plan runs the terraform plan command.
	Plan(context.Context, io.Writer, io.Writer, *PlanOptions) (int, error)

	// Apply runs the terraform apply command.
	Apply(context.Context, io.Writer, io.Writer, *ApplyOptions) (int, error)

	// Show runs the terraform show command.
	Show(context.Context, io.Writer, io.Writer, *ShowOptions) (int, error)
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

// TerraformBackendConfig represents the terraform backend config block.
type TerraformBackendConfig struct {
	GCSBucket *string `hcl:"bucket,attr"`
	Prefix    *string `hcl:"prefix,attr"`
}

type TerraformEntrypoint struct {
	Path        string
	BackendFile string
}

// NewTerraformClient creates a new Terraform client.
func NewTerraformClient(workingDir string) *TerraformClient {
	return &TerraformClient{
		workingDir: workingDir,
	}
}

// GetEntrypointDirectories gets all the directories that have Terraform config
// files containing a backend block to be used as an entrypoint module.
func GetEntrypointDirectories(rootDir string) ([]*TerraformEntrypoint, error) {
	matches := make(map[string]*TerraformEntrypoint)
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

		absPath, err := util.PathEvalAbs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for directory %s: %w", rootDir, err)
		}

		matches[absPath] = &TerraformEntrypoint{
			Path:        filepath.Dir(absPath),
			BackendFile: absPath,
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	entrypoints := maps.Values(matches)

	return entrypoints, nil
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

// ParseBackendConfig extracts the backend configuration from the backend block.
func ParseBackendConfig(path string) (*TerraformBackendConfig, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	parser := hclparse.NewParser()
	file, d := parser.ParseHCLFile(path)
	diags = append(diags, d...)

	if d.HasErrors() {
		for _, diag := range d {
			if diag.Summary == "Failed to read file" {
				return nil, d, fmt.Errorf("failed to read file: %s", path)
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
			c := &TerraformBackendConfig{}
			d := gohcl.DecodeBody(innerBlocks[0].Body, nil, c)
			diags = append(diags, d...)
			return c, diags, nil
		}
	}

	return nil, diags, nil
}

// FormatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func FormatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
