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

	"github.com/abcxyz/guardian/pkg/checkterraform"
	"github.com/abcxyz/pkg/cli"
)

// ProviderSourceFlags are the flags that control which Terraform provider
// sources Guardian allows Terraform to install and execute.
type ProviderSourceFlags struct {
	FlagAllowedProviderSources []string
}

// RegisterProviderSourceFlags registers the provider source flags in the given
// flag section.
func (p *ProviderSourceFlags) RegisterProviderSourceFlags(f *cli.FlagSection) {
	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "allowed-provider-sources",
		Target:  &p.FlagAllowedProviderSources,
		Default: []string{},
		Example: "hashicorp/google,hashicorp/google-beta,integrations/github",
		Usage: "The allowlist of Terraform provider sources, in the form " +
			"[HOSTNAME/]NAMESPACE/TYPE. HOSTNAME defaults to registry.terraform.io " +
			"and '*' matches a single segment (e.g. hashicorp/*). Installed " +
			"providers are checked after terraform init and before any provider " +
			"is executed. If unset, all provider sources are allowed.",
	})
}

// ProviderSourceMatcher builds a SourceMatcher from the configured flags.
func (p *ProviderSourceFlags) ProviderSourceMatcher() (*checkterraform.SourceMatcher, error) {
	m, err := checkterraform.NewSourceMatcher(p.FlagAllowedProviderSources)
	if err != nil {
		return nil, fmt.Errorf("invalid -allowed-provider-sources: %w", err)
	}
	return m, nil
}
