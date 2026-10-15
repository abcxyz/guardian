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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Terraform initializes the backend (or the HCP Terraform "cloud" integration)
// as the very first step of terraform init, before any provider or provisioner
// is installed. Backends are compiled into the terraform binary, so they are
// not covered by the provider and provisioner allowlists, yet many backend
// attributes can be used to execute arbitrary commands (e.g. a kubeconfig exec
// plugin, an AWS credential_process) or to send the runner's ambient
// credentials to an attacker-controlled endpoint (e.g. a custom storage
// endpoint). Guardian therefore only allows backend and cloud blocks that set
// attributes from a configurable allowlist, to static literal values.

// backendTypeRe matches a Terraform backend type name.
var backendTypeRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// backendAttributePathRe matches a valid backend attribute path: one or more
// dot-separated Terraform identifiers.
var backendAttributePathRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)*$`)

// cloudBackendType is the pseudo backend type used for terraform { cloud {} }
// blocks in a [BackendAttributeSet].
const cloudBackendType = "cloud"

// BackendAttributeSet is the allowlist of backend types, and of the attributes
// each may set, that terraform backend and cloud blocks in the configuration
// under review may use. Each entry is TYPE/PATH, where TYPE is a backend type
// (or "cloud" for HCP Terraform cloud blocks) and PATH is an attribute, with
// attributes of nested blocks addressed by dotted paths (e.g.
// s3/assume_role.role_arn). Allowing a path also allows everything nested
// below it. TYPE/* allows every attribute and nested block of TYPE, with any
// value; a bare "*" allows every backend. A backend type with no entries is
// not allowed at all, even with an empty block. A nil or empty set allows no
// backends.
type BackendAttributeSet struct {
	// paths maps a backend type to its allowed attribute paths.
	paths map[string]map[string]struct{}
	// all is the set of backend types for which every attribute is allowed.
	all map[string]struct{}
	// allowAll reports whether every backend is allowed.
	allowAll bool
}

// NewBackendAttributeSet builds a BackendAttributeSet from entries. Blank
// entries are ignored. It returns an error if an entry is not TYPE/PATH,
// TYPE/* or *.
func NewBackendAttributeSet(entries []string) (*BackendAttributeSet, error) {
	s := &BackendAttributeSet{
		paths: make(map[string]map[string]struct{}),
		all:   make(map[string]struct{}),
	}
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if e == AllowAllPattern {
			s.allowAll = true
			continue
		}
		typ, path, ok := strings.Cut(e, "/")
		if !ok || !backendTypeRe.MatchString(typ) {
			return nil, fmt.Errorf("invalid backend attribute %q: must be TYPE/ATTRIBUTE (e.g. gcs/bucket), "+
				"TYPE/* to allow every attribute of a backend type, or * to allow every backend", raw)
		}
		if path == AllowAllPattern {
			s.all[typ] = struct{}{}
			continue
		}
		if !backendAttributePathRe.MatchString(path) {
			return nil, fmt.Errorf("invalid backend attribute %q: attribute must be a dot-separated list of "+
				"identifiers (e.g. s3/assume_role.role_arn)", raw)
		}
		if s.paths[typ] == nil {
			s.paths[typ] = make(map[string]struct{})
		}
		s.paths[typ][path] = struct{}{}
	}
	return s, nil
}

// AllowsType reports whether backend type typ has any entry in the set.
func (s *BackendAttributeSet) AllowsType(typ string) bool {
	if s == nil {
		return false
	}
	if s.allowAll {
		return true
	}
	if _, ok := s.all[typ]; ok {
		return true
	}
	return len(s.paths[typ]) > 0
}

// AllowsAll reports whether every attribute of backend type typ is allowed,
// with any value.
func (s *BackendAttributeSet) AllowsAll(typ string) bool {
	if s == nil {
		return false
	}
	if s.allowAll {
		return true
	}
	_, ok := s.all[typ]
	return ok
}

// Allowed reports whether the attribute at path of backend type typ, or any
// block that contains it, is in the set.
func (s *BackendAttributeSet) Allowed(typ, path string) bool {
	if s.AllowsAll(typ) {
		return true
	}
	if s == nil {
		return false
	}
	paths := s.paths[typ]
	for {
		if _, ok := paths[path]; ok {
			return true
		}
		i := strings.LastIndex(path, ".")
		if i < 0 {
			return false
		}
		path = path[:i]
	}
}

// allowsBelow reports whether the set contains any path of typ nested below
// path.
func (s *BackendAttributeSet) allowsBelow(typ, path string) bool {
	if s == nil {
		return false
	}
	for p := range s.paths[typ] {
		if strings.HasPrefix(p, path+".") {
			return true
		}
	}
	return false
}

// Entries returns the sorted entries in the set, in TYPE/PATH form.
func (s *BackendAttributeSet) Entries() []string {
	if s == nil {
		return nil
	}
	n := len(s.all)
	for _, paths := range s.paths {
		n += len(paths)
	}
	if s.allowAll {
		n++
	}
	if n == 0 {
		return nil
	}
	out := make([]string, 0, n)
	if s.allowAll {
		out = append(out, AllowAllPattern)
	}
	for typ := range s.all {
		out = append(out, typ+"/"+AllowAllPattern)
	}
	for typ, paths := range s.paths {
		for p := range paths {
			out = append(out, typ+"/"+p)
		}
	}
	slices.Sort(out)
	return out
}

// TypeEntries returns the sorted entries in the set for backend type typ.
func (s *BackendAttributeSet) TypeEntries(typ string) []string {
	var out []string
	for _, e := range s.Entries() {
		if e == AllowAllPattern || strings.HasPrefix(e, typ+"/") {
			out = append(out, e)
		}
	}
	return out
}

// checkRootModuleBackends checks every backend and cloud block in the root
// module in dir against allowed. Only the root module is checked because
// Terraform ignores backend and cloud blocks in child modules.
func checkRootModuleBackends(dir string, allowed *BackendAttributeSet) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	parser := hclparse.NewParser()
	var violations []string
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
			return fmt.Errorf("failed to parse %q: %w", p, diags)
		}
		if f == nil {
			return fmt.Errorf("failed to parse %q", p)
		}

		v, err := fileBackendViolations(f.Body, allowed)
		if err != nil {
			return fmt.Errorf("failed to analyze backend configuration in %q: %w", p, err)
		}
		violations = append(violations, v...)
	}

	if len(violations) > 0 {
		allowedMsg := "none"
		if entries := allowed.Entries(); len(entries) > 0 {
			allowedMsg = strings.Join(entries, ", ")
		}
		return fmt.Errorf("terraform backend configuration is not allowed:\n  %s\n"+
			"allowed backend attributes: %s\n"+
			"To allow an attribute, add TYPE/ATTRIBUTE (e.g. gcs/bucket) to -allowed-backend-attributes. "+
			"TYPE/* allows every attribute of that backend type, including ones that run programs or send the "+
			"runner's credentials elsewhere during terraform init, so only use it if you accept that risk for "+
			"the configurations in this repository",
			strings.Join(violations, "\n  "), allowedMsg)
	}
	return nil
}

// fileBackendViolations returns a description of every disallowed attribute
// in the backend and cloud blocks of the terraform blocks in body. It returns
// an error if the configuration cannot be decoded.
func fileBackendViolations(body hcl.Body, allowed *BackendAttributeSet) ([]string, error) {
	content, _, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "terraform"}},
	})
	if diags.HasErrors() {
		return nil, diags
	}
	if content == nil {
		return nil, fmt.Errorf("failed to decode configuration body")
	}

	var violations []string
	for _, tf := range content.Blocks {
		tfContent, _, diags := tf.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "backend", LabelNames: []string{"type"}},
				{Type: "cloud"},
			},
		})
		if diags.HasErrors() {
			return nil, diags
		}
		if tfContent == nil {
			return nil, fmt.Errorf("failed to decode terraform block at %s", tf.DefRange)
		}

		for _, b := range tfContent.Blocks {
			typ := cloudBackendType
			name := "cloud"
			if b.Type == "backend" {
				if len(b.Labels) < 1 || b.Labels[0] == "" {
					return nil, fmt.Errorf("backend block at %s has no type", b.DefRange)
				}
				typ = b.Labels[0]
				name = fmt.Sprintf("backend %q", typ)
			}
			if !allowed.AllowsType(typ) {
				violations = append(violations, fmt.Sprintf("%s block at %s: backend type %q is not allowed", name, b.DefRange, typ))
				continue
			}
			if allowed.AllowsAll(typ) {
				// TYPE/* accepts the block as is, including nested blocks and
				// non-literal values.
				continue
			}
			v, err := bodyViolations(b.Body, typ, "", allowed)
			if err != nil {
				return nil, fmt.Errorf("invalid %s block at %s: %w", name, b.DefRange, err)
			}
			for _, msg := range v {
				violations = append(violations, fmt.Sprintf("%s block at %s: %s", name, b.DefRange, msg))
			}
		}
	}
	return violations, nil
}

// bodyViolations checks every attribute and nested block in body, whose
// attribute paths are prefixed with prefix.
func bodyViolations(body hcl.Body, typ, prefix string, allowed *BackendAttributeSet) ([]string, error) {
	var violations []string
	var attrs hcl.Attributes

	// Native syntax bodies distinguish attributes from blocks. JSON bodies do
	// not, so all of their properties are returned as attributes, and nested
	// objects are walked by valueViolations instead.
	if syntaxBody, ok := body.(*hclsyntax.Body); ok {
		for _, b := range syntaxBody.Blocks {
			path := prefix + b.Type
			if len(b.Labels) > 0 {
				return nil, fmt.Errorf("unexpected labels on nested block %q at %s", path, b.DefRange())
			}
			if !allowed.Allowed(typ, path) && !allowed.allowsBelow(typ, path) {
				violations = append(violations, fmt.Sprintf("block %q is not allowed", path))
				continue
			}
			v, err := bodyViolations(b.Body, typ, path+".", allowed)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}

		attrs = make(hcl.Attributes, len(syntaxBody.Attributes))
		for name, attr := range syntaxBody.Attributes {
			attrs[name] = attr.AsHCLAttribute()
		}
	} else {
		var diags hcl.Diagnostics
		attrs, diags = body.JustAttributes()
		if diags.HasErrors() {
			return nil, diags
		}
	}

	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		attr := attrs[name]
		path := prefix + name
		if !allowed.Allowed(typ, path) && !allowed.allowsBelow(typ, path) {
			violations = append(violations, fmt.Sprintf("attribute %q is not allowed", path))
			continue
		}

		// With a nil EvalContext, variables and function calls are errors, so
		// only static literals can be evaluated.
		v, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q at %s must be a static literal value: %w", path, attr.Range, diags)
		}
		vv, err := valueViolations(v, typ, path, allowed)
		if err != nil {
			return nil, fmt.Errorf("attribute %q at %s: %w", path, attr.Range, err)
		}
		violations = append(violations, vv...)
	}
	return violations, nil
}

// valueViolations checks a literal value at path. If path is allowed, the
// value only needs to be a static literal. Otherwise, only an object (a JSON
// nested block) or a list of objects (a JSON list of nested blocks) whose
// contents are allowed are accepted.
func valueViolations(v cty.Value, typ, path string, allowed *BackendAttributeSet) ([]string, error) {
	if !v.IsWhollyKnown() {
		return nil, fmt.Errorf("must be a static literal value")
	}
	if err := checkNoTemplates(v); err != nil {
		return nil, err
	}
	if allowed.Allowed(typ, path) {
		return nil, nil
	}

	ty := v.Type()
	switch {
	case v.IsNull():
		return []string{fmt.Sprintf("attribute %q is not allowed", path)}, nil
	case ty.IsObjectType() || ty.IsMapType():
		var violations []string
		keys := make([]string, 0, v.LengthInt())
		vals := make(map[string]cty.Value, v.LengthInt())
		for it := v.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			keys = append(keys, k.AsString())
			vals[k.AsString()] = ev
		}
		slices.Sort(keys)
		for _, k := range keys {
			childPath := path + "." + k
			if !allowed.Allowed(typ, childPath) && !allowed.allowsBelow(typ, childPath) {
				violations = append(violations, fmt.Sprintf("attribute %q is not allowed", childPath))
				continue
			}
			vv, err := valueViolations(vals[k], typ, childPath, allowed)
			if err != nil {
				return nil, err
			}
			violations = append(violations, vv...)
		}
		return violations, nil
	case ty.IsTupleType() || ty.IsListType():
		var violations []string
		for it := v.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			if ev.IsNull() || (!ev.Type().IsObjectType() && !ev.Type().IsMapType()) {
				return []string{fmt.Sprintf("attribute %q is not allowed", path)}, nil
			}
			vv, err := valueViolations(ev, typ, path, allowed)
			if err != nil {
				return nil, err
			}
			violations = append(violations, vv...)
		}
		return violations, nil
	default:
		return []string{fmt.Sprintf("attribute %q is not allowed", path)}, nil
	}
}

// checkNoTemplates returns an error if any string in v contains a template
// sequence. JSON strings are only treated as templates when evaluated with a
// non-nil EvalContext, so template sequences are rejected explicitly instead
// of relying on how Terraform evaluates backend configuration.
func checkNoTemplates(v cty.Value) error {
	var found bool
	if err := cty.Walk(v, func(_ cty.Path, ev cty.Value) (bool, error) {
		if !ev.IsNull() && ev.IsKnown() && ev.Type() == cty.String {
			if s := ev.AsString(); strings.Contains(s, "${") || strings.Contains(s, "%{") {
				found = true
			}
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("failed to walk value: %w", err)
	}
	if found {
		return fmt.Errorf("must be a static literal value without template sequences")
	}
	return nil
}
