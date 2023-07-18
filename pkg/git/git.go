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

// Package git defines the functionality to interact with the git CLI.
package git

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/abcxyz/guardian/pkg/child"
	"golang.org/x/exp/maps"
)

var _ Git = (*GitClient)(nil)

// newline is a regexp to split strings at line breaks.
var newline = regexp.MustCompile("\r?\n")

// Git defined the common git functionality.
type Git interface {
	// DiffDirsAbs returns the directories changed using the git diff command
	DiffDirsAbs(ctx context.Context, baseRef, headRef string) ([]string, error)
}

// GitClient implements the git interface.
type GitClient struct {
	workingDir string
}

// NewGitClient creates a new Terraform client.
func NewGitClient(workingDir string) *GitClient {
	return &GitClient{
		workingDir: workingDir,
	}
}

// DiffDirsAbs runs a git diff between two revisions and returns the sorted list
// of absolute directory paths that have changes.
func (g *GitClient) DiffDirsAbs(ctx context.Context, ref1, ref2 string) ([]string, error) {
	var stdout, stderr bytes.Buffer

	_, err := child.Run(ctx, &child.RunConfig{
		Stdout:     &stdout,
		Stderr:     &stderr,
		WorkingDir: g.workingDir,
		Command:    "git",
		Args:       []string{"diff", fmt.Sprintf("%s..%s", ref1, ref2), "--name-only"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run git diff command: %w\n\n%s", err, stderr.String())
	}

	return parseSortedDiffDirsAbs(stdout.String())
}

// parseSortedDiffDirs splits a string at newlines and returns the sorted set of
// absolute directory paths.
func parseSortedDiffDirsAbs(v string) ([]string, error) {
	matches := make(map[string]struct{})

	for _, line := range newline.Split(v, -1) {
		if len(line) > 0 {
			dir := filepath.Dir(line)

			path, err := filepath.Abs(dir)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for directory %s: %w", dir, err)
			}

			matches[path] = struct{}{}
		}
	}

	dirs := maps.Keys(matches)

	sort.Strings(dirs)

	return dirs, nil
}
