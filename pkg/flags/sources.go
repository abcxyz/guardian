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

package flags

import (
	"fmt"
	"slices"

	"github.com/abcxyz/guardian/pkg/checkterraform"
	"github.com/abcxyz/pkg/cli"
)

// DefaultAllowedProviderSources is the default value of
// -allowed-provider-sources. It allows every provider in the hashicorp
// namespace on the public registry, which keeps Guardian cloud platform
// agnostic. Note that it also allows hashicorp/external, which can execute
// arbitrary programs during plan.
var DefaultAllowedProviderSources = []string{"hashicorp/*"}

// SourceFlags are the flags that control which Terraform provider and
// provisioner sources Guardian allows Terraform to install and execute.
type SourceFlags struct {
	FlagAllowedProviderSources    []string
	FlagAllowedProvisionerSources []string
}

// RegisterSourceFlags registers the provider and provisioner source flags in
// the given flag section.
func (p *SourceFlags) RegisterSourceFlags(f *cli.FlagSection) {
	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-provider-sources",
		Target:  &p.FlagAllowedProviderSources,
		Default: slices.Clone(DefaultAllowedProviderSources),
		Example: "hashicorp/google,hashicorp/google-beta,integrations/github",
		Usage: "The allowlist of Terraform provider sources, in the form " +
			"[HOSTNAME/]NAMESPACE/TYPE. HOSTNAME defaults to registry.terraform.io " +
			"and '*' matches a single segment (e.g. hashicorp/*). A bare '*' " +
			"allows every provider source, and an empty value allows none. " +
			"Installed providers are checked after terraform init and before any " +
			"provider is executed.",
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-provisioner-sources",
		Target:  &p.FlagAllowedProvisionerSources,
		Default: []string{},
		Example: "builtin/local-exec",
		Usage: "The allowlist of Terraform provisioner sources, in the form " +
			"[ORIGIN/]TYPE, where ORIGIN is builtin (file, local-exec, remote-exec) " +
			"or plugin, and defaults to builtin. '*' matches a single segment " +
			"(e.g. builtin/*) and a bare '*' allows every provisioner. Provisioners " +
			"are discouraged; plugin provisioners should only be used for quick " +
			"proofs of concept. If unset, no provisioners are allowed.",
	})
}

// ProviderSourceMatcher builds a SourceMatcher from the configured flags.
func (p *SourceFlags) ProviderSourceMatcher() (*checkterraform.SourceMatcher, error) {
	m, err := checkterraform.NewSourceMatcher(p.FlagAllowedProviderSources)
	if err != nil {
		return nil, fmt.Errorf("invalid -allowed-provider-sources: %w", err)
	}
	return m, nil
}

// ProvisionerSourceMatcher builds a ProvisionerSourceMatcher from the
// configured flags.
func (p *SourceFlags) ProvisionerSourceMatcher() (*checkterraform.ProvisionerSourceMatcher, error) {
	m, err := checkterraform.NewProvisionerSourceMatcher(p.FlagAllowedProvisionerSources)
	if err != nil {
		return nil, fmt.Errorf("invalid -allowed-provisioner-sources: %w", err)
	}
	return m, nil
}
