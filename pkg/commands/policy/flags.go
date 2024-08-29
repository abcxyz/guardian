// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"errors"
	"fmt"

	"github.com/abcxyz/pkg/cli"
)

type EnforceFlags struct {
	ResultsFile string
}

func (e *EnforceFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("ENFORCE OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "results-file",
		Example: "results.json",
		Target:  &e.ResultsFile,
		Usage:   "The path to a JSON file containing the OPA eval result.",
	})

	set.AfterParse(func(existingError error) (merr error) {
		if e.ResultsFile == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: results-file is required"))
		}
		return merr
	})
}

type FetchDataFlags struct {
	flagOutputDir string
}

func (fd *FetchDataFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("FETCH-DATA OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "output-dir",
		Example: "example/dir",
		Target:  &fd.flagOutputDir,
		Usage:   "Write the policy data JSON file to a target local directory.",
	})
}
