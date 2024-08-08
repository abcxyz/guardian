package policy

import (
	"fmt"

	"github.com/sethvargo/go-githubactions"
)

// GitHubParams defines the required values sourced from the GitHub context.
type GitHubParams struct {
	Owner             string
	Repository        string
	PullRequestNumber int
}

// FromGitHubContext retrieves the required params from the GitHub context.
func (g *GitHubParams) FromGitHubContext(gctx *githubactions.GitHubContext) error {
	owner, repo := gctx.Repo()
	if owner == "" {
		return fmt.Errorf("failed to get the repository owner")
	}
	if repo == "" {
		return fmt.Errorf("failed to get the repository name")
	}

	number, found := gctx.Event["number"]
	if !found {
		return fmt.Errorf("failed to get pull request number")
	}
	pr, ok := number.(int)
	if !ok {
		return fmt.Errorf("pull request number is not of type int")
	}

	g.Owner = owner
	g.Repository = repo
	g.PullRequestNumber = pr

	return nil
}
