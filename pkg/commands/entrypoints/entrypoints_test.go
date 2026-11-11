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

package entrypoints

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestEntrypointsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(t.Context(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name              string
		flagDir           []string
		flagGuardianDirs  []string
		flagDestRef       string
		flagSourceRef     string
		flagDetectChanges bool
		flagMaxDepth      int
		newGitClient      func(ctx context.Context, dir string) git.Git
		platformClient    *platform.MockPlatform
		err               string
		expStdout         string
		expStderr         string
	}{
		{
			name:              "success",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1"),
						filepath.Join(cwd, "testdata/entrypoint1/project2"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2"]`,
		},
		{
			name:              "success_destroy",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1"),
						filepath.Join(cwd, "testdata/entrypoint1/project2"),
						filepath.Join(cwd, "testdata/entrypoint1/project3"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2"]`,
		},
		{
			name:              "success_multi",
			flagDir:           []string{"testdata/entrypoint1", "testdata/entrypoint2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				var diffResp []string

				if strings.HasSuffix(filepath.ToSlash(dir), "testdata/entrypoint1") {
					diffResp = []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1"),
						filepath.Join(cwd, "testdata/entrypoint1/project2"),
					}
				}

				if strings.HasSuffix(filepath.ToSlash(dir), "testdata/entrypoint2") {
					diffResp = []string{
						filepath.Join(cwd, "testdata/entrypoint2/project3"),
						filepath.Join(cwd, "testdata/entrypoint2/project4"),
					}
				}

				return &git.MockGitClient{
					DiffResp: diffResp,
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2","testdata/entrypoint2/project3","testdata/entrypoint2/project4"]`,
		},
		{
			name:              "success_multi_destroy",
			flagDir:           []string{"testdata/entrypoint1", "testdata/entrypoint2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				var diffResp []string

				if strings.HasSuffix(filepath.ToSlash(dir), "testdata/entrypoint1") {
					diffResp = []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1"),
						filepath.Join(cwd, "testdata/entrypoint1/project3"),
					}
				}

				if strings.HasSuffix(filepath.ToSlash(dir), "testdata/entrypoint2") {
					diffResp = []string{
						filepath.Join(cwd, "testdata/entrypoint2/project4"),
						filepath.Join(cwd, "testdata/entrypoint2/project5"),
					}
				}

				return &git.MockGitClient{
					DiffResp: diffResp,
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint2/project4"]`,
		},
		{
			name:              "returns_json",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1"),
						filepath.Join(cwd, "testdata/entrypoint1/project2"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2"]`,
		},
		{
			name:              "changes_in_entrypoint_subdirectory",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1/files"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1"]`,
		},
		{
			name:              "changes_in_deep_entrypoint_subdirectory",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1/files/a/b/c"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1"]`,
		},
		{
			name:              "changes_outside_any_entrypoint",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1"),
						filepath.Join(cwd, "testdata"),
					},
				}
			},
			expStdout: `[]`,
		},
		{
			name:              "changes_in_module_dir_returns_only_entrypoints",
			flagDir:           []string{"testdata/with_modules"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/with_modules/modules/m"),
					},
				}
			},
			expStdout: `["testdata/with_modules/project1"]`,
		},
		{
			name:              "changes_in_module_subdirectory_returns_only_entrypoints",
			flagDir:           []string{"testdata/with_modules"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/with_modules/modules/m/templates"),
					},
				}
			},
			expStdout: `["testdata/with_modules/project1"]`,
		},
		{
			name:              "nearest_entrypoint_returned",
			flagDir:           []string{"testdata/nested_entrypoints"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/nested_entrypoints/subdir/entrypoint2/subsub"),
					},
				}
			},
			expStdout: `["testdata/nested_entrypoints/subdir/entrypoint2"]`,
		},
		{
			name:              "parent_entrypoint_returned_for_sibling_dir",
			flagDir:           []string{"testdata/nested_entrypoints"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						filepath.Join(cwd, "testdata/nested_entrypoints/subdir"),
					},
				}
			},
			expStdout: `["testdata/nested_entrypoints"]`,
		},
		{
			name:              "changes_with_add_entrypoint",
			flagDir:           []string{"testdata/entrypoint1"},
			flagGuardianDirs:  []string{"testdata/entrypoint1/project1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					// Does not resolve to any entrypoint, so the output comes
					// only from -guardian-dirs.
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1"]`,
		},
		{
			name:              "multi_add_entrypoint",
			flagDir:           []string{"testdata/entrypoint1"},
			flagGuardianDirs:  []string{"testdata/entrypoint1/project1", "testdata/entrypoint1/project2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					// Only project1 is detected as changed, so project1 from
					// -guardian-dirs is deduplicated and project2 is appended.
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1/files/test.txt"),
					},
				}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2"]`,
		},
		{
			name:              "skips_detect_changes",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: false,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{}
			},
			expStdout: `["testdata/entrypoint1/project1","testdata/entrypoint1/project2"]`,
		},
		{
			name:              "errors",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffErr: fmt.Errorf("failed to run git diff"),
				}
			},
			err: "failed to find git diff directories: failed to run git diff",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockPlatformClient := &platform.MockPlatform{}

			c := &EntrypointsCommand{
				flagDir:           tc.flagDir,
				flagGuardianDirs:  tc.flagGuardianDirs,
				flagDestRef:       tc.flagDestRef,
				flagSourceRef:     tc.flagSourceRef,
				flagDetectChanges: tc.flagDetectChanges,
				flagMaxDepth:      tc.flagMaxDepth,
				platformClient:    mockPlatformClient,
				newGitClient:      tc.newGitClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); got != want {
				t.Errorf("expected stdout\n\n%s\n\nto be\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected stderr\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
		})
	}
}

// TestEntrypointsProcess_NoLogsOnStdout guards against log lines corrupting
// the machine-readable JSON written to stdout. Guardian configures its logger
// to write WARNING and above to stdout by default (see cmd/guardian/main.go)
// and the workflows pipe the output of this command into jq, so any
// non-debug log emitted on the happy path breaks every Guardian workflow.
func TestEntrypointsProcess_NoLogsOnStdout(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		diffResp  []string
		expStdout string
	}{
		{
			name: "unmatched_changes",
			diffResp: []string{
				filepath.Join(cwd, "testdata"),
				filepath.Join(cwd, "testdata/entrypoint1"),
				filepath.Join(cwd, "testdata/entrypoint1/project3/does/not/exist"),
			},
			expStdout: "[]\n",
		},
		{
			name: "matched_changes",
			diffResp: []string{
				filepath.Join(cwd, "testdata/entrypoint1/project1/files"),
			},
			expStdout: "[\"testdata/entrypoint1/project1\"]\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &EntrypointsCommand{
				flagDir:           []string{"testdata/entrypoint1"},
				flagDestRef:       "main",
				flagSourceRef:     "ldap/feature",
				flagDetectChanges: true,
				platformClient:    &platform.MockPlatform{},
				newGitClient: func(ctx context.Context, dir string) git.Git {
					return &git.MockGitClient{DiffResp: tc.diffResp}
				},
			}

			_, stdout, _ := c.Pipe()

			// Mirror the production logger configuration, pointed at the
			// same writer as the command's stdout.
			logger := logging.New(stdout, logging.LevelWarning, logging.FormatJSON, false)
			ctx := logging.WithLogger(t.Context(), logger)

			if err := c.Process(ctx); err != nil {
				t.Fatal(err)
			}

			if got, want := stdout.String(), tc.expStdout; got != want {
				t.Errorf("expected stdout to be exactly %q, got %q", want, got)
			}
		})
	}
}

// TestEntrypointsProcess_DetectChangesWithGit runs change detection end to end
// against a real git repository. It covers changes to non-Terraform files in
// subdirectories of entrypoints and modules (e.g. files read via file() or
// fileset()), including modules that are only used transitively, modules
// sourced from outside -dir, and subdirectories that were added or deleted.
//
// The command resolves -dir and writes its output relative to the working
// directory, so this test must chdir and therefore cannot run in parallel.
//
//nolint:paralleltest // t.Chdir cannot be used in parallel tests.
func TestEntrypointsProcess_DetectChangesWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	const backend = `terraform {
  backend "local" {}
}
`

	// Repository layout (arrows point from a caller to the modules it uses):
	//
	//   terraform/app                -> svc, outer, shared/mod
	//   terraform/app/nested            (entrypoint nested in terraform/app)
	//   terraform/app2               -> outer
	//   terraform/modules/outer      -> inner
	//   terraform/modules/unused        (not used by any entrypoint)
	//   shared/mod                      (outside of -dir)
	baseFiles := map[string]string{
		"README.md": "readme\n",
		"terraform/app/main.tf": backend + `
module "svc" {
  source = "../modules/svc"
}

module "outer" {
  source = "../modules/outer"
}

module "shared" {
  source = "../../shared/mod"
}

locals {
  a = file("${path.module}/cfg/a.yaml")
}
`,
		"terraform/app/cfg/a.yaml":     "v1\n",
		"terraform/app/res.tf":         "resource \"null_resource\" \"r\" {}\n",
		"terraform/app/nested/main.tf": backend,
		"terraform/app2/main.tf": backend + `
module "outer" {
  source = "../modules/outer"
}
`,
		"terraform/modules/svc/main.tf": `locals {
  b = file("${path.module}/cfg/b.yaml")
  c = file("${path.module}/cfg/deep/c.yaml")
}
`,
		"terraform/modules/svc/cfg/b.yaml":      "v1\n",
		"terraform/modules/svc/cfg/deep/c.yaml": "v1\n",
		"terraform/modules/outer/main.tf": `module "inner" {
  source = "../inner"
}
`,
		"terraform/modules/inner/main.tf": `locals {
  x = file("${path.module}/cfg/x.yaml")
}
`,
		"terraform/modules/inner/cfg/x.yaml": "v1\n",
		"terraform/modules/unused/main.tf": `locals {
  u = file("${path.module}/cfg/u.yaml")
}
`,
		"terraform/modules/unused/cfg/u.yaml": "v1\n",
		"shared/mod/main.tf": `locals {
  y = file("${path.module}/cfg/y.yaml")
}
`,
		"shared/mod/cfg/y.yaml": "v1\n",
	}

	cases := []struct {
		name      string
		runFrom   string // working directory relative to the repository root
		dir       string // -dir flag, defaults to "terraform"
		write     map[string]string
		remove    []string
		move      [][2]string // {source, destination} pairs moved with git mv
		expStdout string
	}{
		{
			name:      "entrypoint_subdirectory",
			write:     map[string]string{"terraform/app/cfg/a.yaml": "v2\n"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "module_subdirectory",
			write:     map[string]string{"terraform/modules/svc/cfg/b.yaml": "v2\n"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "module_deep_subdirectory",
			write:     map[string]string{"terraform/modules/svc/cfg/deep/c.yaml": "v2\n"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "module_subdirectory_added",
			write:     map[string]string{"terraform/modules/svc/new/n.yaml": "v1\n"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "module_subdirectory_deleted",
			remove:    []string{"terraform/modules/svc/cfg"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "entrypoint_subdirectory_deleted",
			remove:    []string{"terraform/app/cfg"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "entrypoint_deleted",
			remove:    []string{"terraform/app2"},
			expStdout: `[]`,
		},
		{
			// The deleted entrypoint's directory no longer exists, so the
			// change is attributed to the enclosing entrypoint.
			name:      "nested_entrypoint_deleted",
			remove:    []string{"terraform/app/nested"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "transitive_module_subdirectory",
			write:     map[string]string{"terraform/modules/inner/cfg/x.yaml": "v2\n"},
			expStdout: `["terraform/app","terraform/app2"]`,
		},
		{
			name:      "module_outside_dir_subdirectory",
			write:     map[string]string{"shared/mod/cfg/y.yaml": "v2\n"},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "unused_module_subdirectory",
			write:     map[string]string{"terraform/modules/unused/cfg/u.yaml": "v2\n"},
			expStdout: `[]`,
		},
		{
			name:      "unrelated_file",
			write:     map[string]string{"README.md": "updated\n"},
			expStdout: `[]`,
		},
		{
			name:      "file_moved_out_of_entrypoint_subdirectory",
			move:      [][2]string{{"terraform/app/cfg/a.yaml", "a.yaml"}},
			expStdout: `["terraform/app"]`,
		},
		{
			name:      "file_moved_between_entrypoints",
			move:      [][2]string{{"terraform/app/res.tf", "terraform/app2/res.tf"}},
			expStdout: `["terraform/app","terraform/app2"]`,
		},
		{
			name:      "run_from_subdirectory_entrypoint_subdirectory",
			runFrom:   "terraform",
			dir:       ".",
			write:     map[string]string{"terraform/app/cfg/a.yaml": "v2\n"},
			expStdout: `["app"]`,
		},
		{
			name:      "run_from_subdirectory_module_outside_working_dir",
			runFrom:   "terraform",
			dir:       ".",
			write:     map[string]string{"shared/mod/cfg/y.yaml": "v2\n"},
			expStdout: `["app"]`,
		},
		{
			name:      "run_from_subdirectory_unrelated_file",
			runFrom:   "terraform/app",
			dir:       ".",
			write:     map[string]string{"README.md": "updated\n"},
			expStdout: `[]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Avoid logging.TestLogger: it records DEBUG logs, and the real git
			// client logs the entire process environment at DEBUG.
			logger := logging.New(os.Stderr, logging.LevelWarning, logging.FormatJSON, false)
			ctx := logging.WithLogger(t.Context(), logger)

			// Resolve symlinks (e.g. /tmp on macOS) so paths match the ones
			// computed by the command.
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}

			writeTestFiles(t, root, baseFiles)
			runTestGit(t, root, "init", "--quiet")
			runTestGit(t, root, "add", "--all")
			runTestGit(t, root, "commit", "--quiet", "--message", "base")
			baseRef := strings.TrimSpace(runTestGit(t, root, "rev-parse", "HEAD"))

			writeTestFiles(t, root, tc.write)
			for _, p := range tc.remove {
				if err := os.RemoveAll(filepath.Join(root, p)); err != nil {
					t.Fatal(err)
				}
			}
			for _, m := range tc.move {
				runTestGit(t, root, "mv", m[0], m[1])
			}
			runTestGit(t, root, "add", "--all")
			runTestGit(t, root, "commit", "--quiet", "--message", "change")

			t.Chdir(filepath.Join(root, filepath.FromSlash(tc.runFrom)))

			dir := tc.dir
			if dir == "" {
				dir = "terraform"
			}

			c := &EntrypointsCommand{
				flagDir:           []string{dir},
				flagSourceRef:     baseRef,
				flagDestRef:       "HEAD",
				flagDetectChanges: true,
				flagSkipReporting: true,
				platformClient:    &platform.MockPlatform{},
				newGitClient: func(ctx context.Context, dir string) git.Git {
					return git.NewGitClient(dir)
				},
			}

			_, stdout, _ := c.Pipe()

			if err := c.Process(ctx); err != nil {
				t.Fatal(err)
			}

			if got, want := strings.TrimSpace(stdout.String()), tc.expStdout; got != want {
				t.Errorf("expected stdout\n\n%s\n\nto be\n\n%s\n\n", got, want)
			}
		})
	}
}

// writeTestFiles writes files, keyed by slash-separated path relative to root.
func writeTestFiles(tb testing.TB, root string, files map[string]string) {
	tb.Helper()

	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			tb.Fatal(err)
		}
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

func TestAfterParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "validate_refs",
			args: []string{"-detect-changes", "-max-depth=0"},
			err:  "invalid flag: source-ref and dest-ref are required to detect changes, to ignore changes set the detect-changes flag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := EntrypointsCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Error(diff)
			}
		})
	}
}
