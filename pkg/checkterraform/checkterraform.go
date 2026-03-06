// Copyright 2026 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package checkterraform provides the Terraform check functionality for Guardian.
package checkterraform

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"

	"github.com/abcxyz/pkg/logging"
)

const (
	// DefaultDisallowedProviders is the default list of disallowed providers.
	DefaultDisallowedProviders = "external"
)

type ScanResult struct {
	Providers           []string
	Provisioners        []string
	InvalidProviders    []string
	InvalidProvisioners []string
}

// moduleJSON is the structure of the .terraform/modules/modules.json file.
type moduleJSON struct {
	Modules []struct {
		Key    string `json:"Key"`
		Source string `json:"Source"`
		Dir    string `json:"Dir"`
	} `json:"Modules"`
}

// findInvalid returns the list of items in actual that are not in allowed, or if allowed is empty,
// the list of items in actual that are in disallowed.
func findInvalid(allowed, disallowed, actual []string) []string {
	if len(allowed) == 0 && len(disallowed) == 0 {
		return []string{}
	}
	result := slices.Clone(actual)
	if len(allowed) > 0 {
		return slices.DeleteFunc(result, func(s string) bool { return slices.Contains(allowed, s) })
	}
	return slices.DeleteFunc(result, func(s string) bool { return !slices.Contains(disallowed, s) })
}

// CheckProvidersProvisioners returns the result of parsing the terraform content in the given directory (or file).
// The usual case is to provide disallowed providers and provisioners. This is more permissive, only
// erroring if those specifically denied matches occur. If allowedProviders or allowedProvisioners is set,
// they will enforce that *only* those providres or provisioners are present.
// ex. CheckProvidersProvisioners("file.tf", []string{"external"}, []string{"local-exec", "remote-exec"}, nil, nil).
func CheckProvidersProvisioners(ctx context.Context, dir string, disallowedProviders, disallowedProvisioners, allowedProviders, allowedProvisioners []string) (ScanResult, error) {
	providers, provisioners, err := analyzeDir(ctx, dir)
	if err != nil {
		return ScanResult{}, fmt.Errorf("failed to analyze directory %q: %w", dir, err)
	}

	result := ScanResult{
		Providers:           providers,
		Provisioners:        provisioners,
		InvalidProviders:    findInvalid(allowedProviders, disallowedProviders, providers),
		InvalidProvisioners: findInvalid(allowedProvisioners, disallowedProvisioners, provisioners),
	}

	if len(result.InvalidProviders) > 0 || len(result.InvalidProvisioners) > 0 {
		return result, fmt.Errorf("terraform contains invalid providers: %v, invalid provisioners: %v", result.InvalidProviders, result.InvalidProvisioners)
	}
	return result, nil
}

// Walk the provided directory, returning discovered providers and provisioners within Terraform.
func analyzeDir(ctx context.Context, dir string) ([]string, []string, error) {
	logger := logging.FromContext(ctx)
	parser := hclparse.NewParser()
	providerSet := make(map[string]struct{})
	provisionerSet := make(map[string]struct{})

	pathsToWalk := extractPathsFromModulesJSON(ctx, dir)

	for _, path := range pathsToWalk {
		err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			var f *hcl.File
			var diags hcl.Diagnostics
			if strings.HasSuffix(path, ".tf.json") {
				f, diags = parser.ParseJSONFile(path)
			} else if strings.HasSuffix(path, ".tf") {
				f, diags = parser.ParseHCLFile(path)
			} else {
				// Not a recognized Terraform file, skip.
				return nil
			}

			if diags.HasErrors() {
				// we encounter errors parsing, we can't extract further data.
				logger.ErrorContext(
					ctx,
					"failed to parse terraform file",
					"path", path,
					"diagnostics", diags.Error(),
				)
				return nil // Continue to next file despite errors
			}

			extractFromBody(f.Body, providerSet, provisionerSet)
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to walk directory %q: %w", dir, err)
		}
	}

	providerResults := slices.Sorted(maps.Keys(providerSet))
	provisionerResults := slices.Sorted(maps.Keys(provisionerSet))

	return providerResults, provisionerResults, nil
}

// Given the body of a terraform file, extract the providers and provisioners.
func extractFromBody(body hcl.Body, providerSet, provisionerSet map[string]struct{}) {
	// Partial content schema for top-level blocks we are inspecting.
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "provider", LabelNames: []string{"name"}},
			{Type: "terraform"},
			{Type: "resource", LabelNames: []string{"type", "name"}},
			{Type: "data", LabelNames: []string{"type", "name"}},
			{Type: "module", LabelNames: []string{"name"}},
		},
	}

	// We use PartialContent to ignore unknown blocks (like locals, output,
	// variables which we don't care about for providers yet)
	content, _, _ := body.PartialContent(schema)
	if content == nil {
		return
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case "terraform":
			// Format: terraform { required_providers { ... } }
			extractRequiredProviders(block.Body, providerSet)
		case "provider":
			// Format: provider "name" { ... }
			if len(block.Labels) > 0 {
				providerSet[block.Labels[0]] = struct{}{}
			}
		case "resource", "data":
			// Format: resource "type" "name" { ... }
			// Format: data "type" "name" { ... }
			// Identify potential external blocks.
			if len(block.Labels) > 0 {
				// consider the first element of the label (a_b_c -> a) to be the name.
				// This might overcollect, but that serves us better than the opposite
				// condition.
				parts := strings.SplitN(block.Labels[0], "_", 2)
				// Skip terraform data blocks.
				if len(parts) > 0 && parts[0] != "terraform" {
					providerSet[parts[0]] = struct{}{}
				}
			}

			// Identify use of provisioners.
			extractProvisioners(block.Body, provisionerSet)
		}
	}
}

// Filtered to a resource or data block scan search for provisioner blocks, adding any found to the provisionerSet.
func extractProvisioners(body hcl.Body, provisionerSet map[string]struct{}) {
	// within a resource, we can have provisioners
	// Schema for provisioner blocks: provisioner "type" { ... }
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "provisioner", LabelNames: []string{"type"}},
		},
	}

	content, _, _ := body.PartialContent(schema)
	if content == nil {
		return
	}
	for _, block := range content.Blocks {
		if block.Type == "provisioner" {
			if len(block.Labels) > 0 {
				pType := block.Labels[0]
				// Add to providerSet to allow checking compliance (e.g. banning local-exec)
				provisionerSet[pType] = struct{}{}
			}
		}
	}
}

// Filtered to a terraform block scan search for required_providers blocks, adding any found to the providerSet.
func extractRequiredProviders(body hcl.Body, providerSet map[string]struct{}) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "required_providers"},
		},
	}
	content, _, _ := body.PartialContent(schema)
	if content == nil {
		return
	}
	for _, block := range content.Blocks {
		if block.Type == "required_providers" {
			// Inside required_providers, we have attributes map.
			// `google = { ... }` or `google = "version"` (older/legacy)
			attrs, _ := block.Body.JustAttributes()
			for name := range attrs {
				providerSet[name] = struct{}{}
			}
		}
	}
}

// extractPathsFromModulesJSON extracts the paths to walk from modules.json if present.
func extractPathsFromModulesJSON(ctx context.Context, dir string) []string {
	logger := logging.FromContext(ctx)
	pathsToWalk := []string{dir}

	modulesJSONPath := filepath.Join(dir, ".terraform", "modules", "modules.json")
	if content, err := os.ReadFile(modulesJSONPath); err == nil {
		var modules moduleJSON
		if err := json.Unmarshal(content, &modules); err != nil {
			logger.WarnContext(
				ctx,
				"failed to parse modules.json",
				"path",
				modulesJSONPath,
				"error",
				err)
		} else {
			for _, m := range modules.Modules {
				if m.Dir != "" && m.Dir != "." {
					fullPath := filepath.Join(dir, m.Dir)
					if !slices.Contains(pathsToWalk, fullPath) {
						pathsToWalk = append(pathsToWalk, fullPath)
					}
				}
			}
		}
	}

	return pathsToWalk
}
