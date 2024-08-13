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

package util

import (
	"fmt"

	"github.com/abcxyz/pkg/cli"
)

type GuardianBaseCommand struct {
	cli.BaseCommand
}

// OutHeaderf is a shortcut to write a heading to [GuardianBaseCommand.Stdout].
func (c *GuardianBaseCommand) OutHeaderf(format string, a ...any) {
	fmt.Fprintf(c.Stdout(), "\n--------------------\n"+format+"\n--------------------\n", a...)
}
