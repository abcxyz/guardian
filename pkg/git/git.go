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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/exp/maps"

	"github.com/abcxyz/guardian/pkg/child"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/logging"
)

var _ Git = (*GitClient)(nil)

// newline is a regexp to split strings at line breaks.
var newline = regexp.MustCompile("\r?\n")

// Git defined the common git functionality.
type Git interface {
	// DiffDirsAbs returns the directories changed using the git diff command
	DiffDirsAbs(ctx context.Context, baseRef, headRef string) ([]string, error)
	// CloneRepository clones the repository to the workingDir.
	CloneRepository(ctx context.Context, githubToken, owner, repo string) error
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
func (g *GitClient) DiffDirsAbs(ctx context.Context, sourceRef, destRef string) ([]string, error) {
	logger := logging.FromContext(ctx).With("working_dir", g.workingDir)

	// git diff prints paths relative to the root of the repository regardless
	// of the directory it runs in, so resolve them against that root rather
	// than the process working directory.
	root, err := g.repositoryRoot(ctx)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer

	// Disable rename detection: with it, a moved file is listed only at its
	// destination, so the directory it was moved out of would never be
	// considered changed.
	_, err = child.Run(ctx, &child.RunConfig{
		Stdout:     &stdout,
		Stderr:     &stderr,
		WorkingDir: g.workingDir,
		Command:    "git",
		Args:       []string{"diff", "--no-renames", fmt.Sprintf("%s..%s", sourceRef, destRef), "--name-only"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run git diff command: %w\n\n%s", err, stderr.String())
	}

	logger.DebugContext(ctx, "DiffDirsAbs git diff output", "output", stdout.String())

	return parseSortedDiffDirsAbs(ctx, root, stdout.String())
}

// repositoryRoot returns the absolute, symlink-free path to the root of the
// repository containing the working directory.
func (g *GitClient) repositoryRoot(ctx context.Context) (string, error) {
	var stdout, stderr bytes.Buffer

	_, err := child.Run(ctx, &child.RunConfig{
		Stdout:     &stdout,
		Stderr:     &stderr,
		WorkingDir: g.workingDir,
		Command:    "git",
		Args:       []string{"rev-parse", "--show-toplevel"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to find git repository root: %w\n\n%s", err, stderr.String())
	}

	root, err := util.PathEvalAbs(strings.TrimSpace(stdout.String()))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for git repository root: %w", err)
	}
	return root, nil
}

// CloneRepository clones the repository to the workingDir.
func (g *GitClient) CloneRepository(ctx context.Context, githubToken, owner, repo string) error {
	logger := logging.FromContext(ctx).With("working_dir", g.workingDir)

	var stdout, stderr bytes.Buffer

	_, err := child.Run(ctx, &child.RunConfig{
		Stdout:     &stdout,
		Stderr:     &stderr,
		WorkingDir: g.workingDir,
		Command:    "git",
		Args:       []string{"clone", fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", githubToken, owner, repo), "--single-branch", "--no-tags"},
	})
	if err != nil {
		return fmt.Errorf("failed to run git clone command: %w\n\n%s", err, stderr.String())
	}

	logger.DebugContext(ctx, "CloneRepository git clone output", "output", stdout.String())

	return nil
}

// parseSortedDiffDirsAbs splits a string at newlines and returns the sorted set
// of absolute directory paths. Each line is a path relative to root. A changed
// file whose directory no longer exists (because the whole directory was
// deleted) is attributed to its nearest ancestor that still exists, stopping
// at root, so that removing the last file under an entrypoint or module still
// triggers the entrypoint.
func parseSortedDiffDirsAbs(ctx context.Context, root, stdout string) ([]string, error) {
	logger := logging.FromContext(ctx)

	matches := make(map[string]struct{})

	for _, line := range newline.Split(stdout, -1) {
		if len(line) > 0 {
			dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(line)))

			path, err := nearestExistingDir(root, dir)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for directory %s: %w", dir, err)
			}

			matches[path] = struct{}{}
		}
	}

	dirs := maps.Keys(matches)

	sort.Strings(dirs)

	logger.DebugContext(ctx, "parseSortedDiffDirsAbs result", "dirs", dirs)

	return dirs, nil
}

// nearestExistingDir returns the absolute, symlink-free path of dir or, if dir
// no longer exists as a directory, of its nearest ancestor that does, stopping
// at root. A path whose parent was replaced by a regular file fails with
// ENOTDIR rather than ErrNotExist, and a directory replaced by a regular file
// resolves successfully but is not a directory; both are treated as missing.
func nearestExistingDir(root, dir string) (string, error) {
	for {
		path, err := util.PathEvalAbs(dir)
		if err == nil {
			info, statErr := os.Stat(path)
			if statErr != nil {
				return "", fmt.Errorf("failed to stat %s: %w", path, statErr)
			}
			if info.IsDir() {
				return path, nil
			}
		} else if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return "", fmt.Errorf("failed to resolve %s: %w", dir, err)
		}

		if dir == root {
			return "", fmt.Errorf("repository root %s is not a directory", root)
		}
		dir = filepath.Dir(dir)
	}
}
