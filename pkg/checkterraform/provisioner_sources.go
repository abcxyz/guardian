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

// ProvisionerOrigin describes how Terraform loads a provisioner.
type ProvisionerOrigin string

const (
	// ProvisionerOriginBuiltin is a provisioner compiled into Terraform.
	ProvisionerOriginBuiltin ProvisionerOrigin = "builtin"

	// ProvisionerOriginPlugin is a provisioner loaded from a
	// terraform-provisioner-<type> plugin binary.
	ProvisionerOriginPlugin ProvisionerOrigin = "plugin"
)

// builtinProvisioners are the provisioners compiled into Terraform.
var builtinProvisioners = []string{"file", "local-exec", "remote-exec"}

// IsBuiltinProvisioner reports whether typ is a provisioner compiled into
// Terraform.
func IsBuiltinProvisioner(typ string) bool {
	return slices.Contains(builtinProvisioners, typ)
}

// ProvisionerSource identifies a provisioner and how Terraform loads it.
type ProvisionerSource struct {
	Origin ProvisionerOrigin
	Type   string
}

// String returns the provisioner source in the form "<origin>/<type>", e.g.
// "builtin/local-exec".
func (p ProvisionerSource) String() string {
	return string(p.Origin) + "/" + p.Type
}

// ProvisionerSourceForType returns the source of a provisioner block with the
// given type. Built-in provisioner names are classified as built-in and every
// other name as a plugin.
//
// Terraform lets a plugin override a built-in provisioner. Guardian prevents
// the configuration under review from supplying plugins (see
// [RemoveLocalPluginOverrides]), so such an override can only come from the
// trusted environment and is still classified as built-in.
func ProvisionerSourceForType(typ string) *ProvisionerSource {
	if IsBuiltinProvisioner(typ) {
		return &ProvisionerSource{Origin: ProvisionerOriginBuiltin, Type: typ}
	}
	return &ProvisionerSource{Origin: ProvisionerOriginPlugin, Type: typ}
}

// ParseProvisionerSource parses a provisioner source in the form
// [ORIGIN/]TYPE, where ORIGIN is "builtin" or "plugin". A bare TYPE implies
// "builtin". Wildcard characters are not permitted.
func ParseProvisionerSource(s string) (*ProvisionerSource, error) {
	segs, err := splitProvisionerSource(s)
	if err != nil {
		return nil, err
	}
	for _, seg := range segs {
		if strings.ContainsAny(seg, `*?[]\`) {
			return nil, fmt.Errorf("invalid provisioner source %q: wildcards are not permitted", s)
		}
	}

	origin := ProvisionerOrigin(segs[0])
	if origin != ProvisionerOriginBuiltin && origin != ProvisionerOriginPlugin {
		return nil, fmt.Errorf("invalid provisioner source %q: origin must be %q or %q",
			s, ProvisionerOriginBuiltin, ProvisionerOriginPlugin)
	}
	if origin == ProvisionerOriginBuiltin && !IsBuiltinProvisioner(segs[1]) {
		return nil, newNotBuiltinProvisionerError(s, segs[1])
	}
	return &ProvisionerSource{Origin: origin, Type: segs[1]}, nil
}

// splitProvisionerSource splits a provisioner source or pattern into its
// origin and type segments.
func splitProvisionerSource(s string) ([2]string, error) {
	var out [2]string

	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return out, fmt.Errorf("provisioner source must not be empty")
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		out = [2]string{string(ProvisionerOriginBuiltin), parts[0]}
	case 2:
		out = [2]string{parts[0], parts[1]}
	default:
		return out, fmt.Errorf("invalid provisioner source %q: expected [ORIGIN/]TYPE", s)
	}

	for _, seg := range out {
		if seg == "" || strings.ContainsAny(seg, " \t\r\n") {
			return out, fmt.Errorf("invalid provisioner source %q: segments must be non-empty and contain no whitespace", s)
		}
	}
	return out, nil
}

func newNotBuiltinProvisionerError(s, typ string) error {
	return fmt.Errorf("invalid provisioner source %q: %q is not a Terraform built-in provisioner (%s); "+
		"use %s/%s for a plugin", s, typ, strings.Join(builtinProvisioners, ", "), ProvisionerOriginPlugin, typ)
}

// ProvisionerSourceMatcher is an allowlist of provisioner source patterns.
// Patterns have the form [ORIGIN/]TYPE, where ORIGIN is "builtin", "plugin" or
// "*", and a bare TYPE implies "builtin". Patterns are matched with
// [path.Match], so "*" within a pattern matches exactly one segment (e.g.
// "builtin/*"). A bare [AllowAllPattern] allows every provisioner.
//
// A provisioner source is allowed if and only if it matches at least one
// pattern. A matcher with no patterns allows nothing.
type ProvisionerSourceMatcher struct {
	patterns []string
	allowAll bool
}

// NewProvisionerSourceMatcher builds a ProvisionerSourceMatcher from the given
// patterns. Blank patterns are ignored. If no non-blank patterns are given, the
// returned matcher denies every provisioner.
func NewProvisionerSourceMatcher(patterns []string) (*ProvisionerSourceMatcher, error) {
	m := &ProvisionerSourceMatcher{}
	for _, raw := range patterns {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if trimmed == AllowAllPattern {
			m.allowAll = true
			if !slices.Contains(m.patterns, AllowAllPattern) {
				m.patterns = append(m.patterns, AllowAllPattern)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "!") {
			return nil, fmt.Errorf("invalid provisioner source pattern %q: exclusions are not supported, list only the allowed sources", raw)
		}

		segs, err := splitProvisionerSource(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid provisioner source pattern: %w", err)
		}

		switch ProvisionerOrigin(segs[0]) {
		case ProvisionerOriginBuiltin:
			// A literal built-in pattern that names a non-built-in provisioner can
			// never match, which is almost certainly a mistake.
			if !strings.ContainsAny(segs[1], `*?[]\`) && !IsBuiltinProvisioner(segs[1]) {
				return nil, newNotBuiltinProvisionerError(raw, segs[1])
			}
		case ProvisionerOriginPlugin, AllowAllPattern:
		default:
			return nil, fmt.Errorf("invalid provisioner source pattern %q: origin must be %q, %q or %q",
				raw, ProvisionerOriginBuiltin, ProvisionerOriginPlugin, AllowAllPattern)
		}

		pattern := segs[0] + "/" + segs[1]

		// path.Match only reports malformed patterns when matching, so validate
		// eagerly to surface configuration errors up front.
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid provisioner source pattern %q: %w", raw, err)
		}

		if !slices.Contains(m.patterns, pattern) {
			m.patterns = append(m.patterns, pattern)
		}
	}
	return m, nil
}

// Patterns returns the normalized allowlist patterns.
func (m *ProvisionerSourceMatcher) Patterns() []string {
	if m == nil {
		return nil
	}
	return slices.Clone(m.patterns)
}

// Allowed reports whether the given provisioner source is permitted. A nil
// source is never allowed, and a nil or empty matcher allows nothing.
func (m *ProvisionerSourceMatcher) Allowed(p *ProvisionerSource) bool {
	if p == nil || m == nil {
		return false
	}
	if m.allowAll {
		return true
	}
	s := p.String()
	for _, pattern := range m.patterns {
		// Patterns were validated in NewProvisionerSourceMatcher, so the error is
		// always nil.
		if ok, _ := path.Match(pattern, s); ok {
			return true
		}
	}
	return false
}
