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

// Package checkterraform provides the Terraform check functionality for Guardian.
package checkterraform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/abcxyz/pkg/logging"
)

// moduleJSON is the structure of the .terraform/modules/modules.json file.
type moduleJSON struct {
	Modules []struct {
		Key    string `json:"Key"`
		Source string `json:"Source"`
		Dir    string `json:"Dir"`
	} `json:"Modules"`
}

// extractPathsFromModulesJSON extracts the paths to walk from modules.json if present.
func extractPathsFromModulesJSON(ctx context.Context, dir string) []string {
	logger := logging.FromContext(ctx)
	pathsToWalk := []string{dir}

	modulesJSONPath := filepath.Join(dir, ".terraform", "modules", "modules.json")
	content, err := os.ReadFile(modulesJSONPath)
	if err != nil {
		logger.WarnContext(
			ctx,
			"failed to read modules.json",
			"path",
			modulesJSONPath,
			"error",
			err,
		)
	}

	var modules moduleJSON
	if err := json.Unmarshal(content, &modules); err != nil {
		logger.WarnContext(
			ctx,
			"failed to parse modules.json",
			"path",
			modulesJSONPath,
			"error",
			err)
		return pathsToWalk
	}

	for _, m := range modules.Modules {
		if m.Dir != "" && m.Dir != "." {
			fullPath := filepath.Join(dir, m.Dir)
			if !slices.Contains(pathsToWalk, fullPath) {
				pathsToWalk = append(pathsToWalk, fullPath)
			}
		}
	}

	return pathsToWalk
}
