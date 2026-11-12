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

package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/abcxyz/pkg/testutil"
)

func TestParseSortedDiffDirsAbs(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		root  string
		value string
		exp   []string
		err   string
	}{
		{
			name: "success",
			value: `testdata/first/test.txt
testdata/second/test.txt
testdata/third/test.txt`,
			exp: []string{
				filepath.Join(cwd, "testdata/first"),
				filepath.Join(cwd, "testdata/second"),
				filepath.Join(cwd, "testdata/third"),
			},
		},
		{
			name: "resolves_missing_dir_to_nearest_existing_ancestor",
			value: `testdata/first/test.txt
testdata/second/test.txt
testdata/third/test.txt
testdata/fourth/test.txt`,
			exp: []string{
				filepath.Join(cwd, "testdata"),
				filepath.Join(cwd, "testdata/first"),
				filepath.Join(cwd, "testdata/second"),
				filepath.Join(cwd, "testdata/third"),
			},
		},
		{
			name: "resolves_deeply_missing_dir_to_nearest_existing_ancestor",
			value: `testdata/first/gone/also-gone/test.txt
testdata/second/test.txt`,
			exp: []string{
				filepath.Join(cwd, "testdata/first"),
				filepath.Join(cwd, "testdata/second"),
			},
		},
		{
			name:  "resolves_missing_top_level_dir_to_root",
			value: `gone/test.txt`,
			exp:   []string{cwd},
		},
		{
			name: "resolves_relative_to_root",
			root: filepath.Join(cwd, "testdata"),
			value: `first/test.txt
second/test.txt`,
			exp: []string{
				filepath.Join(cwd, "testdata/first"),
				filepath.Join(cwd, "testdata/second"),
			},
		},
		{
			name:  "missing_dir_walk_stops_at_root",
			root:  filepath.Join(cwd, "testdata"),
			value: `gone/also-gone/test.txt`,
			exp:   []string{filepath.Join(cwd, "testdata")},
		},
		{
			name:  "path_under_regular_file_resolves_to_existing_dir",
			value: `testdata/first/test.txt/nested/test.txt`,
			exp:   []string{filepath.Join(cwd, "testdata/first")},
		},
		{
			name:  "regular_file_resolves_to_parent_dir",
			value: `testdata/first/test.txt/test.txt`,
			exp:   []string{filepath.Join(cwd, "testdata/first")},
		},
		{
			name:  "carriage_return_and_newline",
			value: "testdata/first/test.txt\r\ntestdata/third/test.txt\r\ntestdata/second/test.txt",
			exp: []string{
				filepath.Join(cwd, "testdata/first"),
				filepath.Join(cwd, "testdata/second"),
				filepath.Join(cwd, "testdata/third"),
			},
		},
		{
			name: "handles_dirs",
			value: `testdata/first
testdata/second
testdata/third`,
			exp: []string{filepath.Join(cwd, "testdata")},
		},
		{
			name:  "handles_empty",
			value: "",
			exp:   []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := tc.root
			if root == "" {
				root = cwd
			}

			dirs, err := parseSortedDiffDirsAbs(t.Context(), root, tc.value)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if diff := cmp.Diff(dirs, tc.exp); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestDiffDirsAbs(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	cases := []struct {
		name       string
		workingDir string
		change     func(tb testing.TB, root string)
		exp        []string
	}{
		{
			name:       "from_repository_root",
			workingDir: ".",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				writeTestFile(tb, root, "a/b/file.txt", "v2\n")
			},
			exp: []string{"a/b"},
		},
		{
			name:       "from_subdirectory_resolves_against_repository_root",
			workingDir: "a",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				writeTestFile(tb, root, "c/file.txt", "v2\n")
			},
			exp: []string{"c"},
		},
		{
			name:       "from_subdirectory_deleted_dir_stops_at_repository_root",
			workingDir: "a/b",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.RemoveAll(filepath.Join(root, "c")); err != nil {
					tb.Fatal(err)
				}
			},
			exp: []string{"."},
		},
		{
			name:       "dir_replaced_by_file",
			workingDir: ".",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.RemoveAll(filepath.Join(root, "a/b")); err != nil {
					tb.Fatal(err)
				}
				writeTestFile(tb, root, "a/b", "now a file\n")
			},
			exp: []string{"a"},
		},
		{
			name:       "dir_with_nested_dir_replaced_by_file",
			workingDir: ".",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.RemoveAll(filepath.Join(root, "a")); err != nil {
					tb.Fatal(err)
				}
				writeTestFile(tb, root, "a", "now a file\n")
			},
			exp: []string{"."},
		},
		{
			name:       "renamed_file_lists_source_and_destination",
			workingDir: ".",
			change: func(tb testing.TB, root string) {
				tb.Helper()
				runTestGit(tb, root, "mv", "a/b/file.txt", "c/moved.txt")
			},
			exp: []string{"a/b", "c"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Resolve symlinks (e.g. /tmp on macOS) so paths match the ones
			// returned by DiffDirsAbs.
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			writeTestFile(t, root, "a/b/file.txt", "v1\n")
			writeTestFile(t, root, "a/b/keep.txt", "v1\n")
			writeTestFile(t, root, "c/file.txt", "v1\n")
			runTestGit(t, root, "init", "--quiet")
			runTestGit(t, root, "add", "--all")
			runTestGit(t, root, "commit", "--quiet", "--message", "base")
			baseRef := strings.TrimSpace(runTestGit(t, root, "rev-parse", "HEAD"))

			tc.change(t, root)
			runTestGit(t, root, "add", "--all")
			runTestGit(t, root, "commit", "--quiet", "--message", "change")

			client := NewGitClient(filepath.Join(root, filepath.FromSlash(tc.workingDir)))
			got, err := client.DiffDirsAbs(t.Context(), baseRef, "HEAD")
			if err != nil {
				t.Fatal(err)
			}

			exp := make([]string, 0, len(tc.exp))
			for _, p := range tc.exp {
				exp = append(exp, filepath.Join(root, filepath.FromSlash(p)))
			}
			if diff := cmp.Diff(exp, got); diff != "" {
				t.Errorf("DiffDirsAbs (-want, +got):\n%s", diff)
			}
		})
	}
}

// writeTestFile writes a file at a slash-separated path relative to root.
func writeTestFile(tb testing.TB, root, name, contents string) {
	tb.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		tb.Fatal(err)
	}
}

// runTestGit runs git in dir, isolated from the user's and system's git
// configuration, and returns its stdout.
func runTestGit(tb testing.TB, dir string, args ...string) string {
	tb.Helper()

	cmd := exec.CommandContext(tb.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)

	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
		tb.Fatalf("git %s failed: %v\n\n%s", strings.Join(args, " "), err, stderr)
	}
	return string(out)
}
