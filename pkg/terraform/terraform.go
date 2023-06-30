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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/abcxyz/guardian/pkg/child"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

var _ Terraform = (*TerraformClient)(nil)

// Terraform is the interface for working with the Terraform CLI.
type Terraform interface {
	Init(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error)
	Validate(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error)
	Plan(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error)
	Apply(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error)
	Show(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error)
	GetEntrypointDirectories(rootDir string) ([]string, error)
}

// TerraformClient implements the Terraform interface.
type TerraformClient struct {
	runner child.Runner
}

// NewTerraformClient creates a new Terraform client.
func NewTerraformClient() *TerraformClient {
	runner := &child.ChildRunner{}
	return &TerraformClient{
		runner: runner,
	}
}

// Init runs the terraform init command.
func (t *TerraformClient) Init(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error) {
	childArgs := []string{"init"}
	childArgs = append(childArgs, args...)
	return t.runner.Run(ctx, workingDir, "terraform", childArgs) //nolint:wrapcheck
}

// Validate runs the terraform validate command.
func (t *TerraformClient) Validate(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error) {
	childArgs := []string{"validate"}
	childArgs = append(childArgs, args...)
	return t.runner.Run(ctx, workingDir, "terraform", childArgs) //nolint:wrapcheck
}

// Show runs the Terraform show command.
func (t *TerraformClient) Show(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	childArgs := []string{"show"}
	childArgs = append(childArgs, args...)
	childArgs = append(childArgs, file)
	return t.runner.Run(ctx, workingDir, "terraform", childArgs) //nolint:wrapcheck
}

// Plan runs the Terraform plan command.
func (t *TerraformClient) Plan(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	childArgs := []string{"plan"}
	childArgs = append(childArgs, args...)
	childArgs = append(childArgs, fmt.Sprintf("-out=%s", file))
	return t.runner.Run(ctx, workingDir, "terraform", childArgs) //nolint:wrapcheck
}

// Apply runs the Terraform plan command.
func (t *TerraformClient) Apply(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	childArgs := []string{"apply"}
	childArgs = append(childArgs, args...)
	childArgs = append(childArgs, file)
	return t.runner.Run(ctx, workingDir, "terraform", childArgs) //nolint:wrapcheck
}

// FormatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func FormatOutputForGitHubDiff(content string) string {
	changed := regexp.MustCompile("(?m)^([\t ]*)([~])")
	createReplace := regexp.MustCompile(`(?m)^([\t ]*)((\-(\/\+)*)|(\+(\/\-)*)|(!))`)

	content = changed.ReplaceAllString(content, `$1!`)
	content = createReplace.ReplaceAllString(content, "$2$1")

	return content
}

// GetEntrypointDirectories gets all the directories that have Terraform config
// files containing a backend block to be used as an entrypoint module.
func (t *TerraformClient) GetEntrypointDirectories(rootDir string) ([]string, error) {
	matches := make(map[string]struct{})
	if err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", path, err)
		}

		if d.IsDir() {
			return nil
		}

		matched, err := filepath.Match("*.tf", filepath.Base(path))
		if err != nil {
			return fmt.Errorf("failed to find terraform files: %w", err)
		}

		if !matched {
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

	dirs := []string{}

	for dir := range matches {
		dirs = append(dirs, dir)
	}

	sort.Strings(dirs)

	return dirs, nil
}

// hasBackendConfig tests a Terraform config file for the existence of a backend block.
func hasBackendConfig(path string) (bool, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	if _, err := os.Stat(path); err != nil {
		return false, nil, fmt.Errorf("failed to find file: %w", err)
	}

	parser := hclparse.NewParser()
	file, d := parser.ParseHCLFile(path)
	diags = append(diags, d...)

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
