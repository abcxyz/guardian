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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/abcxyz/pkg/logging"

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

// InitRequiredCommands are the Terraform commands that
// require terraform init to be run first.
var InitRequiredCommands = map[string]struct{}{
	"validate":  {},
	"plan":      {},
	"apply":     {},
	"destroy":   {},
	"console":   {},
	"graph":     {},
	"import":    {},
	"output":    {},
	"providers": {},
	"refresh":   {},
	"show":      {},
	"state":     {},
	"taint":     {},
	"untaint":   {},
	"workspace": {},
}

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

	// Run runs a terraform command.
	Run(context.Context, io.Writer, io.Writer, string, ...string) (int, error)
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
	// GCSBucket is the GCS Bucket used in a terraform backend config block.
	GCSBucket *string `hcl:"bucket,attr"`
	// Prefix is the GCS Bucket prefix used in a terraform backend config block.
	Prefix *string `hcl:"prefix,attr"`
}

// TerraformEntrypoint describes a terraform entrypoint.
type TerraformEntrypoint struct {
	// Path is the filesystem path to the entrypoint.
	Path string
	// BackendFile is the filename of the terraform file that contains the backend config block.
	BackendFile string
}

// ModuleUsageGraph describes which entrypoints use which modules and which modules are used by each entrypoint.
type ModuleUsageGraph struct {
	// EntrypointToModules is the complete list of all modules used by a terraform entrypoint at any depth.
	EntrypointToModules map[string]map[string]struct{}
	// ModulesToEntrypoints is the complete list of all terraform entrypoints that use a particular module at any depth.
	ModulesToEntrypoints map[string]map[string]struct{}
}

// NewTerraformClient creates a new Terraform client.
func NewTerraformClient(workingDir string) *TerraformClient {
	return &TerraformClient{
		workingDir: workingDir,
	}
}

// ModuleUsage locates all the usages of modules in all terraform entrypoints and vice versa.
func ModuleUsage(ctx context.Context, rootDir string, maxDepth *int, skipUnresolvableModules bool) (*ModuleUsageGraph, error) {
	entrypoints, err := GetEntrypointDirectories(rootDir, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to get entrypoints: %w", err)
	}
	moduleUsages, err := modules(ctx, rootDir, skipUnresolvableModules)
	if err != nil {
		return nil, fmt.Errorf("failed to get module usages: %w", err)
	}
	entrypointToModules := make(map[string]map[string]struct{})
	modulesToEntrypoints := make(map[string]map[string]struct{})
	for _, entrypoint := range entrypoints {
		entrypointToModules[entrypoint.Path] = make(map[string]struct{})
		recurseAndAppend(entrypoint.Path, entrypoint.Path, entrypointToModules, moduleUsages)
	}
	for _, modules := range entrypointToModules {
		for module := range modules {
			if _, ok := modulesToEntrypoints[module]; !ok {
				modulesToEntrypoints[module] = make(map[string]struct{})
			}
		}
	}
	for module := range modulesToEntrypoints {
		for entrypoint, modules := range entrypointToModules {
			if _, modOK := modules[module]; modOK {
				modulesToEntrypoints[module][entrypoint] = struct{}{}
			}
		}
	}
	return &ModuleUsageGraph{
		EntrypointToModules:  entrypointToModules,
		ModulesToEntrypoints: modulesToEntrypoints,
	}, nil
}

func recurseAndAppend(rootPth, pth string, entrypointToModules map[string]map[string]struct{}, moduleUsages map[string]*Modules) {
	if usage, ok := moduleUsages[pth]; ok {
		for modulePath := range usage.ModulePaths {
			entrypointToModules[rootPth][modulePath] = struct{}{}
			recurseAndAppend(rootPth, modulePath, entrypointToModules, moduleUsages)
		}
	}
}

// Modules represents the details of the modules used by a given module or terraform entrypoint.
type Modules struct {
	// ModulePaths is the list of all modules used in the particular terraform or module entrypoint.
	ModulePaths map[string]struct{}
}

// modules locates all terraform entrypoints or modules and finds all of their module usages.
func modules(ctx context.Context, rootDir string, skipUnresolvableModules bool) (map[string]*Modules, error) {
	logger := logging.FromContext(ctx)
	matches := make(map[string]*Modules)
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

		absPath, err := util.PathEvalAbs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for directory %s: %w", rootDir, err)
		}

		if _, ok := matches[filepath.Dir(absPath)]; !ok {
			matches[filepath.Dir(absPath)] = &Modules{
				ModulePaths: make(map[string]struct{}),
			}
		}
		modulePaths := matches[filepath.Dir(absPath)].ModulePaths
		modules, _, err := ExtractModules(path)
		if err != nil {
			return fmt.Errorf("failed to extract modules: %w", err)
		}

		for m := range modules.ModulePaths {
			relativeModulePath := filepath.Join(filepath.Dir(absPath), m)
			pth, err := util.PathEvalAbs(relativeModulePath)
			if err != nil {
				if skipUnresolvableModules {
					logger.DebugContext(ctx, "Skipping unresolvable module", "module", relativeModulePath, "used_in_tf_file", absPath)
					continue
				}
				return fmt.Errorf("failed to get absolute path for directory %s: %w", relativeModulePath, err)
			}
			modulePaths[pth] = struct{}{}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	return matches, nil
}

// GetEntrypointDirectories gets all the directories that have Terraform config
// files containing a backend block to be used as an entrypoint module.
func GetEntrypointDirectories(rootDir string, maxDepth *int) ([]*TerraformEntrypoint, error) {
	matches := make(map[string]*TerraformEntrypoint)
	startPathSeparatorCount := strings.Count(rootDir, string(os.PathSeparator))
	if err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", path, err)
		}

		if d.IsDir() && maxDepth != nil && strings.Count(path, string(os.PathSeparator))-startPathSeparatorCount > *maxDepth {
			return fs.SkipDir
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
	sort.Slice(entrypoints, func(i, j int) bool {
		return entrypoints[i].Path < entrypoints[j].Path
	})

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

// ExtractBackendConfig extracts the backend configuration from the backend block.
func ExtractBackendConfig(path string) (*TerraformBackendConfig, hcl.Diagnostics, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	return extractBackendConfig(b, path)
}

func extractBackendConfig(contents []byte, filename string) (*TerraformBackendConfig, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	parser := hclparse.NewParser()
	file, d := parser.ParseHCL(contents, filename)
	diags = append(diags, d...)

	if d.HasErrors() {
		for _, diag := range d {
			if diag.Summary == "Failed to parse file" {
				return nil, d, fmt.Errorf("failed to read file: %s", filename)
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

// ExtractModules extracts the details about the modules used in a particular file.
func ExtractModules(path string) (*Modules, hcl.Diagnostics, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	return extractModules(b, path)
}

func extractModules(contents []byte, filename string) (*Modules, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	parser := hclparse.NewParser()
	file, d := parser.ParseHCL(contents, filename)
	diags = append(diags, d...)

	if d.HasErrors() {
		for _, diag := range d {
			if diag.Summary == "Failed to parse file" {
				return nil, d, fmt.Errorf("failed to read file: %s", filename)
			}
		}
	}

	rootBlocks, _, d := file.Body.PartialContent(RootSchema)
	diags = append(diags, d...)
	paths := make(map[string]struct{})

	for _, terraformBlock := range rootBlocks.Blocks.OfType("module") {
		content, _, d := terraformBlock.Body.PartialContent(ModuleSchema)
		diags = append(diags, d...)
		v, d := content.Attributes["source"].Expr.Value(nil)
		diags = append(diags, d...)
		paths[v.AsString()] = struct{}{}
	}

	return &Modules{ModulePaths: paths}, diags, nil
}

// FormatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func FormatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
