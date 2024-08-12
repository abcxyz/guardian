// Copyright 2023 The Authors (see AUTHORS file)
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
	"errors"
	"fmt"
	"os"

	"github.com/abcxyz/pkg/cli"
)

var allowedPlatforms = map[string]struct{}{
	"local":  {},
	"github": {},
}

type GlobalFlags struct {
	FlagPlatform string
}

func (g *GlobalFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("GLOBAL OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "platform",
		Target:  &g.FlagPlatform,
		Example: "github",
		Usage:   "The code review platform for Guardian to integrate with.",
	})

	set.AfterParse(func(merr error) error {
		switch g.FlagPlatform {
		case "github":
		default:
			if os.Getenv("GITHUB_ACTIONS") == "true" {
				g.FlagPlatform = "github"
				break
			}

			g.FlagPlatform = "local"
		}

		if _, ok := allowedPlatforms[g.FlagPlatform]; !ok {
			merr = errors.Join(merr, fmt.Errorf("unsupported value for platform flag: %s", g.FlagPlatform))
		}

		return merr
	})
}
