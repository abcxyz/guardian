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
	"errors"
	"fmt"
	"io/fs"
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

// extractPathsFromModulesJSON returns dir followed by the module directories
// listed in dir/.terraform/modules/modules.json, if that file exists.
//
// A missing modules.json (e.g. before terraform init) is not an error. A
// modules.json that cannot be read or parsed is an error, so that callers fail
// closed rather than silently skipping every installed module.
func extractPathsFromModulesJSON(ctx context.Context, dir string) ([]string, error) {
	logger := logging.FromContext(ctx)
	pathsToWalk := []string{dir}

	modulesJSONPath := filepath.Join(dir, ".terraform", "modules", "modules.json")
	content, err := os.ReadFile(modulesJSONPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.DebugContext(ctx, "modules.json not found", "path", modulesJSONPath)
			return pathsToWalk, nil
		}
		return nil, fmt.Errorf("failed to read modules.json: %w", err)
	}

	var modules moduleJSON
	if err := json.Unmarshal(content, &modules); err != nil {
		return nil, fmt.Errorf("failed to parse modules.json: %w", err)
	}

	for _, m := range modules.Modules {
		if m.Dir != "" && m.Dir != "." {
			fullPath := filepath.Join(dir, m.Dir)
			if !slices.Contains(pathsToWalk, fullPath) {
				pathsToWalk = append(pathsToWalk, fullPath)
			}
		}
	}

	return pathsToWalk, nil
}
