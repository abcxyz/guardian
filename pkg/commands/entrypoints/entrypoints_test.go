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
	"path"
	"strings"
	"testing"

	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/guardian/pkg/reporter"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/testutil"
)

func TestEntrypointsProcess(t *testing.T) {
	t.Parallel()

	ctx := logging.WithLogger(context.Background(), logging.TestLogger(t))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name              string
		flagDir           []string
		flagDestRef       string
		flagSourceRef     string
		flagDetectChanges bool
		flagMaxDepth      int
		modifierContent   string
		newGitClient      func(ctx context.Context, dir string) git.Git
		platformClient    *platform.MockPlatform
		reporterClient    *reporter.MockReporter
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
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
					},
				}
			},
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint1/project2"],"destroy":[]}`,
		},
		{
			name:              "success_destroy",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			modifierContent:   "GUARDIAN_DESTROY=testdata/entrypoint1/project3",
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
						path.Join(cwd, "testdata/entrypoint1/project3"),
					},
				}
			},
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint1/project2"],"destroy":["testdata/entrypoint1/project3"]}`,
		},
		{
			name:              "success_destroy_all",
			flagDir:           []string{"testdata/entrypoint1"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			modifierContent:   "GUARDIAN_DESTROY=all",
			newGitClient: func(ctx context.Context, dir string) git.Git {
				return &git.MockGitClient{
					DiffResp: []string{
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
						path.Join(cwd, "testdata/entrypoint1/project3"),
					},
				}
			},
			expStdout: `{"update":[],"destroy":["testdata/entrypoint1/project1","testdata/entrypoint1/project2","testdata/entrypoint1/project3"]}`,
		},
		{
			name:              "success_multi",
			flagDir:           []string{"testdata/entrypoint1", "testdata/entrypoint2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				var diffResp []string

				if strings.HasSuffix(dir, "testdata/entrypoint1") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
					}
				}

				if strings.HasSuffix(dir, "testdata/entrypoint2") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint2/project3"),
						path.Join(cwd, "testdata/entrypoint2/project4"),
					}
				}

				return &git.MockGitClient{
					DiffResp: diffResp,
				}
			},
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint1/project2","testdata/entrypoint2/project3","testdata/entrypoint2/project4"],"destroy":[]}`,
		},
		{
			name:              "success_multi_destroy",
			flagDir:           []string{"testdata/entrypoint1", "testdata/entrypoint2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			modifierContent: `GUARDIAN_DESTROY=testdata/entrypoint1/project3
GUARDIAN_DESTROY=testdata/entrypoint2/project5`,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				var diffResp []string

				if strings.HasSuffix(dir, "testdata/entrypoint1") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project3"),
					}
				}

				if strings.HasSuffix(dir, "testdata/entrypoint2") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint2/project4"),
						path.Join(cwd, "testdata/entrypoint2/project5"),
					}
				}

				return &git.MockGitClient{
					DiffResp: diffResp,
				}
			},
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint2/project4"],"destroy":["testdata/entrypoint1/project3","testdata/entrypoint2/project5"]}`,
		},
		{
			name:              "success_multi_destroy_all",
			flagDir:           []string{"testdata/entrypoint1", "testdata/entrypoint2"},
			flagDestRef:       "main",
			flagSourceRef:     "ldap/feature",
			flagDetectChanges: true,
			modifierContent:   `GUARDIAN_DESTROY=all`,
			newGitClient: func(ctx context.Context, dir string) git.Git {
				var diffResp []string

				if strings.HasSuffix(dir, "testdata/entrypoint1") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
						path.Join(cwd, "testdata/entrypoint1/project3"),
					}
				}

				if strings.HasSuffix(dir, "testdata/entrypoint2") {
					diffResp = []string{
						path.Join(cwd, "testdata/entrypoint2/project3"),
						path.Join(cwd, "testdata/entrypoint2/project4"),
						path.Join(cwd, "testdata/entrypoint2/project5"),
					}
				}

				return &git.MockGitClient{
					DiffResp: diffResp,
				}
			},
			expStdout: `{"update":[],"destroy":["testdata/entrypoint1/project1","testdata/entrypoint1/project2","testdata/entrypoint1/project3","testdata/entrypoint2/project3","testdata/entrypoint2/project4","testdata/entrypoint2/project5"]}`,
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
						path.Join(cwd, "testdata/entrypoint1/project1"),
						path.Join(cwd, "testdata/entrypoint1/project2"),
					},
				}
			},
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint1/project2"],"destroy":[]}`,
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
			expStdout: `{"update":["testdata/entrypoint1/project1","testdata/entrypoint1/project2"],"destroy":[]}`,
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
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockPlatformClient := &platform.MockPlatform{
				ModifierContentResp: tc.modifierContent,
			}
			mockReporterClient := &reporter.MockReporter{}

			c := &EntrypointsCommand{
				flagDir:           tc.flagDir,
				flagDestRef:       tc.flagDestRef,
				flagSourceRef:     tc.flagSourceRef,
				flagDetectChanges: tc.flagDetectChanges,
				flagMaxDepth:      tc.flagMaxDepth,
				platformClient:    mockPlatformClient,
				reporterClient:    mockReporterClient,

				newGitClient: tc.newGitClient,
			}

			_, stdout, stderr := c.Pipe()

			err := c.Process(ctx)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}

			if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(tc.expStdout); !strings.Contains(got, want) {
				t.Errorf("expected stdout\n\n%s\n\nto contain\n\n%s\n\n", got, want)
			}
			if got, want := strings.TrimSpace(stderr.String()), strings.TrimSpace(tc.expStderr); !strings.Contains(got, want) {
				t.Errorf("expected stderr\n\n%s\n\nto contain\n\n%s\n\n", got, want)
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
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := EntrypointsCommand{}

			f := c.Flags()
			err := f.Parse(tc.args)
			if diff := testutil.DiffErrString(err, tc.err); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}
