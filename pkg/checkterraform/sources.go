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
	"path"
	"slices"
	"strings"
)

const (
	// DefaultRegistryHost is the implied registry hostname for provider sources
	// that do not specify one.
	DefaultRegistryHost = "registry.terraform.io"

	// DefaultProviderNamespace is the implied namespace for provider sources
	// that only specify a type.
	DefaultProviderNamespace = "hashicorp"

	builtinHost      = "terraform.io"
	builtinNamespace = "builtin"
)

// ProviderSource is a fully qualified, normalized Terraform provider address.
type ProviderSource struct {
	Hostname  string
	Namespace string
	Type      string
}

// String returns the fully qualified provider source, e.g.
// "registry.terraform.io/hashicorp/google".
func (p ProviderSource) String() string {
	return p.Hostname + "/" + p.Namespace + "/" + p.Type
}

// IsBuiltin reports whether the provider is built into Terraform (e.g.
// terraform.io/builtin/terraform) and therefore is never downloaded.
func (p ProviderSource) IsBuiltin() bool {
	return p.Hostname == builtinHost && p.Namespace == builtinNamespace
}

// ParseProviderSource normalizes a Terraform provider source string using the
// same defaults as Terraform:
//
//	"google"                           -> registry.terraform.io/hashicorp/google
//	"hashicorp/google"                 -> registry.terraform.io/hashicorp/google
//	"registry.terraform.io/x/y"        -> registry.terraform.io/x/y
//
// All segments are lower-cased. Wildcard characters are not permitted.
func ParseProviderSource(s string) (ProviderSource, error) {
	segs, err := splitSource(s)
	if err != nil {
		return ProviderSource{}, err
	}
	for _, seg := range segs {
		if strings.ContainsAny(seg, `*?[]\`) {
			return ProviderSource{}, fmt.Errorf("invalid provider source %q: wildcards are not permitted", s)
		}
	}
	return ProviderSource{Hostname: segs[0], Namespace: segs[1], Type: segs[2]}, nil
}

// splitSource splits and normalizes a provider source or pattern into its
// hostname, namespace and type segments.
func splitSource(s string) ([3]string, error) {
	var out [3]string

	trimmed := strings.ToLower(strings.TrimSpace(s))
	if trimmed == "" {
		return out, fmt.Errorf("provider source must not be empty")
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		out = [3]string{DefaultRegistryHost, DefaultProviderNamespace, parts[0]}
	case 2:
		out = [3]string{DefaultRegistryHost, parts[0], parts[1]}
	case 3:
		out = [3]string{parts[0], parts[1], parts[2]}
	default:
		return out, fmt.Errorf("invalid provider source %q: expected [HOSTNAME/]NAMESPACE/TYPE", s)
	}

	for _, seg := range out {
		if seg == "" || strings.ContainsAny(seg, " \t\r\n") {
			return out, fmt.Errorf("invalid provider source %q: segments must be non-empty and contain no whitespace", s)
		}
	}
	return out, nil
}

// SourceMatcher is an allowlist of provider source patterns. Patterns are
// normalized with the same rules as [ParseProviderSource] and matched with
// [path.Match], so "*" matches exactly one segment and
// "hashicorp/*" is equivalent to "registry.terraform.io/hashicorp/*".
//
// A provider source is allowed if and only if it matches at least one pattern.
type SourceMatcher struct {
	patterns []string
}

// NewSourceMatcher builds a SourceMatcher from the given patterns. Blank
// patterns are ignored. If no non-blank patterns are given, the returned
// matcher is not [SourceMatcher.Enabled] and allows all sources.
func NewSourceMatcher(patterns []string) (*SourceMatcher, error) {
	m := &SourceMatcher{}
	for _, raw := range patterns {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(raw), "!") {
			return nil, fmt.Errorf("invalid provider source pattern %q: exclusions are not supported, list only the allowed sources", raw)
		}

		segs, err := splitSource(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid provider source pattern: %w", err)
		}
		pattern := strings.Join(segs[:], "/")

		// path.Match only reports malformed patterns when matching, so validate
		// eagerly to surface configuration errors up front.
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid provider source pattern %q: %w", raw, err)
		}

		if !slices.Contains(m.patterns, pattern) {
			m.patterns = append(m.patterns, pattern)
		}
	}
	return m, nil
}

// Enabled reports whether any patterns were configured. A matcher that is not
// enabled allows every provider source.
func (m *SourceMatcher) Enabled() bool {
	return m != nil && len(m.patterns) > 0
}

// Patterns returns the normalized allowlist patterns.
func (m *SourceMatcher) Patterns() []string {
	if m == nil {
		return nil
	}
	return slices.Clone(m.patterns)
}

// Allowed reports whether the given provider source is permitted. Built-in
// providers are always allowed because they are never downloaded or executed
// as a separate plugin.
func (m *SourceMatcher) Allowed(p ProviderSource) bool {
	if !m.Enabled() || p.IsBuiltin() {
		return true
	}
	s := p.String()
	for _, pattern := range m.patterns {
		// Patterns were validated in NewSourceMatcher, so the error is always nil.
		if ok, _ := path.Match(pattern, s); ok {
			return true
		}
	}
	return false
}
