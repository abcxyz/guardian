package policy

import (
	"testing"

	"github.com/abcxyz/pkg/testutil"
	"github.com/sethvargo/go-githubactions"
)

func TestParams_FromGitHubContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		gitHubContext *githubactions.GitHubContext
		wantErr       string
	}{
		{
			name: "succeeds_with_valid_repository_and_event",
			gitHubContext: &githubactions.GitHubContext{
				Repository: "test-org/test-repo",
				Event: map[string]any{
					"number": 1,
				},
			},
		},
		{
			name: "succeeds_with_event_owner_and_repository",
			gitHubContext: &githubactions.GitHubContext{
				Event: map[string]any{
					"number": 1,
					"repository": map[string]any{
						"owner": map[string]any{
							"name": "test-org",
						},
						"name": "test-repo",
					},
				},
			},
		},
		{
			name: "fails_with_event_number_not_int_type",
			gitHubContext: &githubactions.GitHubContext{
				Repository: "test-org/test-repo",
				Event: map[string]any{
					"number": "1",
				},
			},
			wantErr: "pull request number is not of type int",
		},
		{
			name: "fails_with_invalid_repository_name",
			gitHubContext: &githubactions.GitHubContext{
				Repository: "test-repo",
				Event: map[string]any{
					"number": 1,
				},
			},
			wantErr: "failed to get the repository name",
		},
		{
			name: "fails_without_repository_name",
			gitHubContext: &githubactions.GitHubContext{
				Event: map[string]any{
					"number": 1,
				},
			},
			wantErr: "failed to get the repository owner",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := &GitHubParams{}
			err := g.FromGitHubContext(tc.gitHubContext)
			if diff := testutil.DiffErrString(err, tc.wantErr); diff != "" {
				t.Errorf("unexpected result; (-got,+want): %s", diff)
			}
		})
	}
}
