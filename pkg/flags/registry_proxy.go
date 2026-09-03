// Copyright 2026 The Authors (see AUTHORS file)
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

package flags

import (
	"github.com/abcxyz/pkg/cli"
)

// RegistryProxyFlags represent the Terraform provider registry proxy flags shared
// among the Terraform commands. Embed this struct into any command that routes
// provider requests through a registry proxy or network mirror.
type RegistryProxyFlags struct {
	// FlagRegistryProxy is the base URL of the Terraform provider registry proxy
	// or network mirror through which provider requests are routed.
	FlagRegistryProxy string
}

// Register registers the registry proxy flags onto the given flag set.
func (r *RegistryProxyFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("REGISTRY PROXY OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "registry-proxy",
		EnvVar:  "GUARDIAN_REGISTRY_PROXY",
		Target:  &r.FlagRegistryProxy,
		Example: "https://localhost:8080/",
		Usage:   "The base URL for a Terraform Provider Registry Proxy or Network Mirror. When set, Guardian routes all provider requests through the proxy, which transparently decides where to fetch each provider from.",
	})
}
