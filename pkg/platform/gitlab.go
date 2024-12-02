package platform

import "context"

var _ Platform = (*GitLab)(nil)

type GitLab struct{}

func NewGitLab(ctx context.Context) (*GitLab, error) {
	return &GitLab{}, nil
}

func (g *GitLab) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	return &AssignReviewersResult{}, nil
}

func (g *GitLab) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	return &GetLatestApproversResult{}, nil
}

func (g *GitLab) GetUserRepoPermissions(ctx context.Context) (string, error) {
	return "", nil
}

func (g *GitLab) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	return []string{}, nil
}

func (g *GitLab) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	return &GetPolicyDataResult{}, nil
}

func (g *GitLab) StoragePrefix(ctx context.Context) (string, error) {
	return "", nil
}
