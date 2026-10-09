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
	"fmt"
	"os"
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
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1/files/test.txt"),
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
					DiffResp: []string{
						filepath.Join(cwd, "testdata/entrypoint1/project1/files/test.txt"),
						filepath.Join(cwd, "testdata/entrypoint1/project2/files/test.txt"),
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
