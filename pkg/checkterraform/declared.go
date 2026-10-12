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

package checkterraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"

	"github.com/abcxyz/pkg/logging"
)

// builtinLocalName is the local name of Terraform's built-in provider, used by
// e.g. terraform_remote_state and terraform_data.
const builtinLocalName = "terraform"

// CheckDeclaredProviderSources statically analyzes the Terraform configuration
// in dir and returns the provider sources it requires. It returns an error if
// any of those sources is not allowed by m.
//
// Each module directory (dir itself, plus any module directories listed in
// .terraform/modules/modules.json) is analyzed independently, mirroring how
// Terraform resolves provider local names per module:
//
//   - required_providers entries with a source use that source.
//   - Any other referenced local name (provider blocks, resource and data
//     source types, and resource "provider" meta-arguments) uses Terraform's
//     implied source, hashicorp/<name>.
//
// Configuration that cannot be parsed or analyzed is an error, so the check
// fails closed. Files that Terraform ignores (e.g. hidden files) are skipped.
//
// This check is intended to run before terraform init so that disallowed
// providers declared in the root module are rejected without being
// downloaded. It is not a substitute for CheckInstalledProviderSources, which
// must still be run after init.
func CheckDeclaredProviderSources(ctx context.Context, dir string, m *SourceMatcher) ([]*ProviderSource, error) {
	logger := logging.FromContext(ctx)

	declared, err := analyzeModules(ctx, dir)
	if err != nil {
		return nil, err
	}

	logger.DebugContext(ctx, "found declared terraform provider sources",
		"dir", dir,
		"providers", declared.providers)

	var disallowed []string
	for _, p := range declared.providers {
		if !m.Allowed(p) {
			disallowed = append(disallowed, p.String())
		}
	}
	if len(disallowed) > 0 {
		return declared.providers, newDisallowedSourcesError(disallowed, m)
	}
	return declared.providers, nil
}

// CheckDeclaredProvisionerSources statically analyzes the Terraform
// configuration in dir, in the same way as [CheckDeclaredProviderSources], and
// returns the sources of the provisioners it uses. It returns an error if any
// of those sources is not allowed by m. If m has no patterns, any provisioner
// is rejected.
//
// Run it before terraform init to reject provisioners in the root module, and
// again after init (before any command that may load provisioners, such as
// validate) to also cover modules installed by init.
func CheckDeclaredProvisionerSources(ctx context.Context, dir string, m *ProvisionerSourceMatcher) ([]*ProvisionerSource, error) {
	logger := logging.FromContext(ctx)

	declared, err := analyzeModules(ctx, dir)
	if err != nil {
		return nil, err
	}

	logger.DebugContext(ctx, "found declared terraform provisioner sources",
		"dir", dir,
		"provisioners", declared.provisioners)

	var disallowed []string
	for _, p := range declared.provisioners {
		if !m.Allowed(p) {
			disallowed = append(disallowed, p.String())
		}
	}
	if len(disallowed) > 0 {
		return declared.provisioners, newDisallowedProvisionerSourcesError(disallowed, m)
	}
	return declared.provisioners, nil
}

// declaredSources are the sources required by Terraform configuration.
type declaredSources struct {
	providers    []*ProviderSource
	provisioners []*ProvisionerSource
}

// analyzeModules analyzes each module directory (dir itself, plus any module
// directories listed in .terraform/modules/modules.json) and returns the
// de-duplicated, sorted sources they require.
func analyzeModules(ctx context.Context, dir string) (*declaredSources, error) {
	providers := make(map[ProviderSource]struct{})
	provisioners := make(map[ProvisionerSource]struct{})
	for _, moduleDir := range extractPathsFromModulesJSON(ctx, dir) {
		sources, err := analyzeModule(moduleDir)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze module directory %q: %w", moduleDir, err)
		}
		for _, s := range sources.providers {
			providers[*s] = struct{}{}
		}
		for _, s := range sources.provisioners {
			provisioners[*s] = struct{}{}
		}
	}

	out := &declaredSources{
		providers:    make([]*ProviderSource, 0, len(providers)),
		provisioners: make([]*ProvisionerSource, 0, len(provisioners)),
	}
	for s := range providers {
		out.providers = append(out.providers, &s)
	}
	slices.SortFunc(out.providers, func(a, b *ProviderSource) int {
		return strings.Compare(a.String(), b.String())
	})
	for s := range provisioners {
		out.provisioners = append(out.provisioners, &s)
	}
	slices.SortFunc(out.provisioners, func(a, b *ProvisionerSource) int {
		return strings.Compare(a.String(), b.String())
	})
	return out, nil
}

// moduleReferences are the provider and provisioner references found in a
// single module.
type moduleReferences struct {
	requiredSources map[string]string // provider local name -> source
	localNames      map[string]struct{}
	provisioners    map[string]struct{} // provisioner types
}

// analyzeModule returns the sources required by the single Terraform module in
// dir. Subdirectories are not traversed because each directory is a separate
// module in Terraform.
//
// Any configuration that cannot be parsed or analyzed is an error rather than
// being skipped, so that the check fails closed.
func analyzeModule(dir string) (*declaredSources, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	parser := hclparse.NewParser()
	refs := &moduleReferences{
		requiredSources: make(map[string]string),
		localNames:      make(map[string]struct{}),
		provisioners:    make(map[string]struct{}),
	}

	for _, e := range entries {
		if e.IsDir() || isIgnoredFile(e.Name()) {
			continue
		}

		p := filepath.Join(dir, e.Name())

		var f *hcl.File
		var diags hcl.Diagnostics
		switch {
		case strings.HasSuffix(p, ".tf.json"):
			f, diags = parser.ParseJSONFile(p)
		case strings.HasSuffix(p, ".tf"):
			f, diags = parser.ParseHCLFile(p)
		default:
			continue
		}
		if diags.HasErrors() {
			return nil, fmt.Errorf("failed to parse %q: %w", p, diags)
		}
		if f == nil {
			return nil, fmt.Errorf("failed to parse %q", p)
		}

		if err := collectReferences(f.Body, refs); err != nil {
			return nil, fmt.Errorf("failed to analyze %q: %w", p, err)
		}
	}

	out := &declaredSources{
		providers:    make([]*ProviderSource, 0, len(refs.localNames)),
		provisioners: make([]*ProvisionerSource, 0, len(refs.provisioners)),
	}
	for name := range refs.localNames {
		raw := name
		if src, ok := refs.requiredSources[name]; ok && src != "" {
			raw = src
		} else if name == builtinLocalName {
			continue
		}

		ps, err := ParseProviderSource(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid provider source for %q: %w", name, err)
		}
		out.providers = append(out.providers, ps)
	}
	for typ := range refs.provisioners {
		out.provisioners = append(out.provisioners, ProvisionerSourceForType(typ))
	}
	return out, nil
}

// isIgnoredFile reports whether Terraform ignores the file with the given name
// when loading a module. This mirrors Terraform's configs.IsIgnoredFile: hidden
// files, editor backup files (foo~) and emacs lock files (#foo#).
func isIgnoredFile(name string) bool {
	return strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "~") ||
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#"))
}

// collectReferences records every provider local name referenced in body,
// every required_providers source, and every provisioner type into refs. It
// returns an error if any of the relevant blocks cannot be decoded.
func collectReferences(body hcl.Body, refs *moduleReferences) error {
	content, _, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "terraform"},
			{Type: "provider", LabelNames: []string{"name"}},
			{Type: "resource", LabelNames: []string{"type", "name"}},
			{Type: "data", LabelNames: []string{"type", "name"}},
			{Type: "ephemeral", LabelNames: []string{"type", "name"}},
		},
	})
	if diags.HasErrors() {
		return diags
	}
	if content == nil {
		return fmt.Errorf("failed to decode configuration body")
	}

	for _, block := range content.Blocks {
		switch block.Type {
		case "terraform":
			if err := collectRequiredProviderSources(block.Body, refs.requiredSources, refs.localNames); err != nil {
				return err
			}
		case "provider":
			if len(block.Labels) == 0 {
				return fmt.Errorf("provider block at %s has no name", block.DefRange)
			}
			refs.localNames[block.Labels[0]] = struct{}{}
		case "resource", "data", "ephemeral":
			if err := collectResourceReferences(block, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectRequiredProviderSources parses terraform { required_providers { ... } }.
func collectRequiredProviderSources(body hcl.Body, requiredSources map[string]string, localNames map[string]struct{}) error {
	content, _, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "required_providers"}},
	})
	if diags.HasErrors() {
		return diags
	}
	if content == nil {
		return fmt.Errorf("failed to decode terraform block")
	}

	for _, block := range content.Blocks {
		attrs, diags := block.Body.JustAttributes()
		if diags.HasErrors() {
			return diags
		}
		for name, attr := range attrs {
			localNames[name] = struct{}{}
			src, err := requiredProviderSource(attr.Expr)
			if err != nil {
				return fmt.Errorf("invalid required_providers entry %q at %s: %w", name, attr.Range, err)
			}
			if src != "" {
				requiredSources[name] = src
			}
		}
	}
	return nil
}

// requiredProviderSource extracts the "source" value from a required_providers
// entry such as `google = { source = "hashicorp/google", version = "..." }`.
// Only the source value is evaluated, so other keys that contain references
// (e.g. configuration_aliases) do not cause a failure. Legacy string-only
// entries (`google = "~> 5.0"`) have no source and return "".
//
// It returns an error if the entry is neither an object nor a string, if the
// source is not a literal string, or if the source is set more than once.
func requiredProviderSource(expr hcl.Expression) (string, error) {
	pairs, diags := hcl.ExprMap(expr)
	if diags.HasErrors() {
		// Not an object, so this must be a legacy version constraint string.
		v, diags := expr.Value(nil)
		if diags.HasErrors() {
			return "", diags
		}
		if v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
			return "", fmt.Errorf("must be an object or a version constraint string")
		}
		return "", nil
	}

	var source string
	var found bool
	for _, pair := range pairs {
		key := hcl.ExprAsKeyword(pair.Key)
		if key == "" {
			v, diags := pair.Key.Value(nil)
			if diags.HasErrors() {
				return "", diags
			}
			if v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
				return "", fmt.Errorf("object keys must be strings")
			}
			key = v.AsString()
		}
		if key != "source" {
			continue
		}
		if found {
			return "", fmt.Errorf("source is set more than once")
		}

		v, diags := pair.Value.Value(nil)
		if diags.HasErrors() {
			return "", diags
		}
		if v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
			return "", fmt.Errorf("source must be a literal string")
		}
		source, found = v.AsString(), true
	}
	return source, nil
}

// collectResourceReferences records the provider local name and provisioner
// types used by a resource, data or ephemeral block into refs. If the block
// sets the "provider" meta-argument, that local name is used; otherwise
// Terraform infers it from the resource type prefix (google_project -> google).
// It returns an error if the "provider" meta-argument is not a provider
// reference or a provisioner block cannot be decoded.
func collectResourceReferences(block *hcl.Block, refs *moduleReferences) error {
	content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "provider"}},
		Blocks:     []hcl.BlockHeaderSchema{{Type: "provisioner", LabelNames: []string{"type"}}},
	})
	if diags.HasErrors() {
		return diags
	}
	if content == nil {
		return fmt.Errorf("failed to decode %s block at %s", block.Type, block.DefRange)
	}

	for _, b := range content.Blocks {
		if len(b.Labels) == 0 || b.Labels[0] == "" {
			return fmt.Errorf("provisioner block at %s has no type", b.DefRange)
		}
		refs.provisioners[b.Labels[0]] = struct{}{}
	}

	if attr, ok := content.Attributes["provider"]; ok {
		trav, diags := hcl.AbsTraversalForExpr(attr.Expr)
		if diags.HasErrors() {
			return diags
		}
		if len(trav) == 0 {
			return fmt.Errorf("invalid provider reference at %s", attr.Range)
		}
		refs.localNames[trav.RootName()] = struct{}{}
		return nil
	}

	if len(block.Labels) == 0 || block.Labels[0] == "" {
		return fmt.Errorf("%s block at %s has no type", block.Type, block.DefRange)
	}
	name, _, _ := strings.Cut(block.Labels[0], "_")
	refs.localNames[name] = struct{}{}
	return nil
}
