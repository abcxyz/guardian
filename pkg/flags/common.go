// Copyright 2024 The Authors (see AUTHORS file)
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

type CommonFlags struct {
	FlagDir          string
	FlagBodyContents string
}

func (c *CommonFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("COMMON OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "dir",
		Target:  &c.FlagDir,
		Example: "./terraform",
		Usage:   "The location of the terraform directory",
	})
}
