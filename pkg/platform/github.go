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

package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/sethvargo/go-retry"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	gh "github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/githubauth"
	"github.com/abcxyz/pkg/logging"
)

const githubMaxCommentLength = 65536

var (
	_ Platform = (*GitHub)(nil)

	// ignoredStatusCodes are status codes that should not be retried. This list
	// is taken from the GitHub REST API documentation.
	ignoredStatusCodes = map[int]struct{}{
		403: {},
		422: {},
	}
)

// GitHub implements the Platform interface.
type GitHub struct {
	cfg           *gh.Config
	client        *github.Client
	graphqlClient *githubv4.Client
	logURL        string
}

// NewGitHub creates a new GitHub client.
func NewGitHub(ctx context.Context, cfg *gh.Config) (*GitHub, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialRetryDelay <= 0 {
		cfg.InitialRetryDelay = 1 * time.Second
	}
	if cfg.MaxRetryDelay <= 0 {
		cfg.MaxRetryDelay = 20 * time.Second
	}

	ghToken := cfg.GuardianGitHubToken
	if ghToken == "" {
		ghToken = cfg.GitHubToken
	}

	var ts oauth2.TokenSource
	if ghToken != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: ghToken,
		})
	} else {
		signer, err := githubauth.NewPrivateKeySigner(cfg.GitHubAppPrivateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		app, err := githubauth.NewApp(cfg.GitHubAppID, signer)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, cfg.GitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		ts = installation.SelectedReposOAuth2TokenSource(ctx, cfg.Permissions, cfg.GitHubRepo)
	}

	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	graphqlClient := githubv4.NewClient(tc)

	g := &GitHub{
		cfg:           cfg,
		client:        client,
		graphqlClient: graphqlClient,
	}

	if cfg.GitHubServerURL != "" || cfg.GitHubRunID > 0 || cfg.GitHubRunAttempt > 0 {
		g.logURL = fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", cfg.GitHubServerURL, cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubRunID, cfg.GitHubRunAttempt)
	}

	if cfg.GitHubJobName != "" {
		resolvedURL, err := g.resolveJobLogsURL(ctx)
		if err == nil {
			g.logURL = resolvedURL
		}
	}

	return g, nil
}

// RequestReviewers abstracts GitHub's RequestReviewers API with retries.
func (g *GitHub) requestReviewers(ctx context.Context, reviewers *github.ReviewersRequest) error {
	if reviewers == nil {
		return fmt.Errorf("reviewer cannot be nil")
	}
	return g.withRetries(ctx, func(ctx context.Context) error {
		if _, resp, err := g.client.PullRequests.RequestReviewers(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber, *reviewers); err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to request reviewers: %w", err)
		}
		return nil
	})
}

// AssignReviewers assigns a list of users and teams as reviewers of a target
// Pull Request. The implementation assigns one principal per request because
// mixing existing reviewers in pending review state with new reviewers will
// result in a no-op without errors thrown.
func (g *GitHub) AssignReviewers(ctx context.Context, input *AssignReviewersInput) (*AssignReviewersResult, error) {
	logger := logging.FromContext(ctx)
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	var result AssignReviewersResult
	for _, u := range input.Users {
		if err := g.requestReviewers(ctx, &github.ReviewersRequest{Reviewers: []string{u}}); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"user", u,
				"error", err,
			)
			continue
		}
		result.Users = append(result.Users, u)
	}
	for _, t := range input.Teams {
		if err := g.requestReviewers(ctx, &github.ReviewersRequest{TeamReviewers: []string{t}}); err != nil {
			logger.ErrorContext(ctx, "failed to assign reviewer for pull request",
				"team", t,
				"error", err,
			)
			continue
		}
		result.Teams = append(result.Teams, t)
	}

	if len(result.Users) == 0 && len(result.Teams) == 0 {
		return nil, fmt.Errorf("failed to assign all requested reviewers to pull request")
	}

	return &result, nil
}

type pullRequestReview struct {
	Author struct {
		Login string
	}
	State string
}

type latestApproverQuery struct {
	Repository struct {
		PullRequest struct {
			LatestReviews struct {
				Nodes []pullRequestReview
			} `graphql:"latestReviews(first: 100)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// GetLatestApprovers retrieves the users whose latest review for a pull request
// is an approval. It also returns the teams and subteams that the user
// approvers are members of to indicate approval on behalf of those teams. Note,
// a comment following a previous approval by the same user will still keep the
// APPROVED state. However, if a reviewer previously approved the PR and
// requests changes/dismisses the review, then the approval is not counted.
func (g *GitHub) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "querying latest approvers")

	var approversQuery latestApproverQuery
	if err := g.graphqlClient.Query(ctx, &approversQuery, map[string]any{
		"owner":  githubv4.String(g.cfg.GitHubOwner),
		"repo":   githubv4.String(g.cfg.GitHubRepo),
		"number": githubv4.Int(g.cfg.GitHubPullRequestNumber),
	}); err != nil {
		return nil, fmt.Errorf("failed to query latest approvers: %w", err)
	}

	// Explicitly sets the default Users and Teams to empty slices. If these are
	// not explicitly provided to OPA, then the policy result may be incorrect.
	result := &GetLatestApproversResult{
		Users: []string{},
	}
	hasApproved := make(map[string]struct{}, len(approversQuery.Repository.PullRequest.LatestReviews.Nodes))
	for _, review := range approversQuery.Repository.PullRequest.LatestReviews.Nodes {
		if review.State == "APPROVED" {
			result.Users = append(result.Users, review.Author.Login)
			hasApproved[review.Author.Login] = struct{}{}
		}
	}

	logger.DebugContext(ctx, "found latest approvers from",
		"users", result.Users,
	)
	if !g.cfg.IncludeTeams {
		logger.DebugContext(ctx, "skipped fetching team approvers")
		return result, nil
	}

	found := make(map[string]struct{})
	for username := range hasApproved {
		teams, err := g.GetUserTeamMemberships(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("failed to get team memberships for approvers: %w", err)
		}
		for _, t := range teams {
			if _, ok := found[t]; !ok {
				result.Teams = append(result.Teams, t)
				found[t] = struct{}{}
			}
		}
	}
	logger.DebugContext(ctx, "found latest approvers from",
		"teams", result.Teams,
	)

	return result, nil
}

type member struct {
	Login string
}

// The members() GraphQL query may include more than one user, if one's username
// is a substring of another's. This is because the 'query' parameter does not
// support exact matches.
type team struct {
	Name    string
	Members struct {
		Nodes []member
	} `graphql:"members(first: 100, query: $username)"`
}

// The teams() GraphQL query does support 'userLogins' parameter, but this was
// observed to only return teams that the users are direct members of. This is
// why we filter by username in the members GraphQL subquery.
type organizationTeamsForUserQuery struct {
	Organization struct {
		Teams struct {
			Nodes []team
		} `graphql:"teams(first: 100)"`
	} `graphql:"organization(login: $owner)"`
}

// GetUserTeamMemberships returns a list of teams that a user is a member of,
// within the given GitHub organization.
func (g *GitHub) GetUserTeamMemberships(ctx context.Context, username string) ([]string, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "querying user team memberships",
		"org", g.cfg.GitHubOwner,
		"user", username)

	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	var teamQuery organizationTeamsForUserQuery
	if err := g.graphqlClient.Query(ctx, &teamQuery, map[string]any{
		"owner":    githubv4.String(g.cfg.GitHubOwner),
		"username": githubv4.String(username),
	}); err != nil {
		return nil, fmt.Errorf("failed to query user team memberships: %w", err)
	}

	res := make([]string, 0)
	for _, team := range teamQuery.Organization.Teams.Nodes {
		for _, m := range team.Members.Nodes {
			// It is important to check for exact matches of the username, due to the
			// lack of exact username matching in the GraphQL query.
			if m.Login == username {
				res = append(res, team.Name)
				break
			}
		}
	}

	return res, nil
}

// GetUserRepoPermissions returns the repo permission for the user that
// triggered the workflow.
func (g *GitHub) GetUserRepoPermissions(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "querying user repo permissions")

	if g.cfg.GitHubActor == "" {
		return "", fmt.Errorf("github-actor is required")
	}
	var result string
	return result, g.withRetries(ctx, func(ctx context.Context) error {
		permissionLevel, resp, err := g.client.Repositories.GetPermissionLevel(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubActor)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to get user repo permissions: %w", err)
		}
		result = *permissionLevel.Permission
		return nil
	})
}

// GitHubActorData defines the payload of the actor used for policy evaluation.
type GitHubActorData struct {
	Username    string   `json:"username"`
	AccessLevel string   `json:"access_level"`
	Teams       []string `json:"teams,omitempty"`
}

// GitHubPolicyData defines the payload of GitHub contextual data used for
// policy evaluation.
type GitHubPolicyData struct {
	PullRequestApprovers *GetLatestApproversResult `json:"pull_request_approvers"`
	Actor                *GitHubActorData          `json:"actor"`
}

// GetPolicyData aggregates data from GitHub into a payload used for policy
// evaluation.
func (g *GitHub) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	p, err := g.GetUserRepoPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permissions: %w", err)
	}

	var actorTeams []string
	if g.cfg.IncludeTeams {
		actorTeams, err = g.GetUserTeamMemberships(ctx, g.cfg.GitHubActor)
		if err != nil {
			return nil, fmt.Errorf("failed to get user teams: %w", err)
		}
	}

	actor := &GitHubActorData{
		Username:    g.cfg.GitHubActor,
		AccessLevel: p,
		Teams:       actorTeams,
	}

	var approvers *GetLatestApproversResult
	// Skip, if the command is not running in the context of a pull request.
	if g.cfg.GitHubPullRequestNumber > 0 {
		approvers, err = g.GetLatestApprovers(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest approvers: %w", err)
		}
	}

	return &GetPolicyDataResult{
		GitHub: &GitHubPolicyData{
			PullRequestApprovers: approvers,
			Actor:                actor,
		},
	}, nil
}

func (g *GitHub) withRetries(ctx context.Context, retryFunc retry.RetryFunc) error {
	backoff := retry.NewFibonacci(g.cfg.InitialRetryDelay)
	backoff = retry.WithMaxRetries(g.cfg.MaxRetries, backoff)
	backoff = retry.WithCappedDuration(g.cfg.MaxRetryDelay, backoff)

	if err := retry.Do(ctx, backoff, retryFunc); err != nil {
		return fmt.Errorf("failed to execute retriable function: %w", err)
	}
	return nil
}

// ModifierContent returns the pull request body as the content to parse modifiers
// from.
func (g *GitHub) ModifierContent(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)

	pullRequestEvents := []string{"pull_request", "pull_request_target"}
	if slices.Contains(pullRequestEvents, g.cfg.GitHubEventName) {
		logger.DebugContext(ctx, "modifier content from pull request",
			"github_pull_request_number", g.cfg.GitHubPullRequestNumber,
			"github_pull_request_body", g.cfg.GitHubPullRequestBody)
		return g.cfg.GitHubPullRequestBody, nil
	}

	pullRequestFromCommitEvents := []string{"push"}
	if slices.Contains(pullRequestFromCommitEvents, g.cfg.GitHubEventName) {
		logger.DebugContext(ctx, "looking up pull request from commit sha",
			"owner", g.cfg.GitHubOwner,
			"repo", g.cfg.GitHubRepo,
			"commit_sha", g.cfg.GitHubSHA)

		var body strings.Builder
		ghPullRequests, err := g.getPullRequestsForCommit(ctx, g.cfg.GitHubSHA, &github.PullRequestListOptions{ListOptions: github.ListOptions{PerPage: 100}})
		if err != nil {
			return "", fmt.Errorf("failed to get pull requests for commit sha: %s", g.cfg.GitHubSHA)
		}

		if len(ghPullRequests) == 0 {
			return "", fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
		}

		for _, v := range ghPullRequests {
			logger.DebugContext(ctx, "found pull request for sha",
				"pull_request_number", v.GetNumber(),
				"pull_request_body", v.GetBody())

			body.WriteString(v.GetBody())
		}

		logger.DebugContext(ctx, "modifier content from commit pull requests",
			"modifier_content", body.String())

		return body.String(), nil
	}

	logger.DebugContext(ctx, "returning no modifier content")
	return "", nil
}

// StoragePrefix generates the unique storage prefix for the github platform type.
func (g *GitHub) StoragePrefix(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)

	pullRequestEvents := []string{"pull_request", "pull_request_target"}
	if slices.Contains(pullRequestEvents, g.cfg.GitHubEventName) {
		var merr error
		if g.cfg.GitHubOwner == "" {
			merr = errors.Join(merr, fmt.Errorf("github owner is required for storage"))
		}
		if g.cfg.GitHubRepo == "" {
			merr = errors.Join(merr, fmt.Errorf("github repo is required for storage"))
		}
		if g.cfg.GitHubPullRequestNumber <= 0 {
			merr = errors.Join(merr, fmt.Errorf("github pull request number is required for storage"))
		}

		if merr != nil {
			return "", merr
		}

		logger.DebugContext(ctx, "storage prefix from pull request event data",
			"github_pull_request_number", g.cfg.GitHubPullRequestNumber,
			"github_pull_request_body", g.cfg.GitHubPullRequestBody)

		return fmt.Sprintf("guardian-plans/%s/%s/%d", g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber), nil
	}

	pullRequestFromCommitEvents := []string{"push"}
	if slices.Contains(pullRequestFromCommitEvents, g.cfg.GitHubEventName) {
		logger.DebugContext(ctx, "looking up pull request from commit sha",
			"owner", g.cfg.GitHubOwner,
			"repo", g.cfg.GitHubRepo,
			"commit_sha", g.cfg.GitHubSHA)

		var prData *github.PullRequest
		result, err := g.getPullRequestsForCommit(ctx, g.cfg.GitHubSHA, &github.PullRequestListOptions{ListOptions: github.ListOptions{PerPage: 100}})
		if err != nil {
			return "", fmt.Errorf("failed to get pull requests for commit sha: %s", g.cfg.GitHubSHA)
		}

		if len(result) == 0 {
			return "", fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
		}
		prData = result[0]

		logger.DebugContext(ctx, "computed pull request number from commit sha",
			"github_pull_request_number", prData.GetNumber())

		return fmt.Sprintf("guardian-plans/%s/%s/%d", g.cfg.GitHubOwner, g.cfg.GitHubRepo, prData.GetNumber()), nil
	}

	logger.DebugContext(ctx, "returning no storage prefix")
	return "", nil
}

// ReportStatus reports the status of a run.
func (g *GitHub) ReportStatus(ctx context.Context, st Status, p *StatusParams) error {
	if err := validateGitHubReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	// Look up PR number if not provided by flag or environment, else status
	// cannot be reported.
	if g.cfg.GitHubPullRequestNumber <= 0 {
		result, err := g.getPullRequestsForCommit(ctx, g.cfg.GitHubSHA, &github.PullRequestListOptions{ListOptions: github.ListOptions{PerPage: 100}})
		if err != nil {
			return fmt.Errorf("failed to get pull request number for commit sha: %s", g.cfg.GitHubSHA)
		}

		if len(result) == 0 {
			return fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
		}

		g.cfg.GitHubPullRequestNumber = result[0].GetNumber()
	}

	msg, err := statusMessage(st, p, g.logURL, githubMaxCommentLength)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	if err = g.createIssueComment(
		ctx,
		g.cfg.GitHubOwner,
		g.cfg.GitHubRepo,
		g.cfg.GitHubPullRequestNumber,
		msg.String(),
	); err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// ReportEntrypointsSummary implements the reporter EntrypointsSummary function by writing a GitHub comment.
func (g *GitHub) ReportEntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	if err := validateGitHubReporterInputs(g.cfg); err != nil {
		return fmt.Errorf("failed to validate reporter inputs: %w", err)
	}

	// Look up PR number if not provided by flag or environment, else status
	// cannot be reported.
	if g.cfg.GitHubPullRequestNumber <= 0 {
		result, err := g.getPullRequestsForCommit(ctx, g.cfg.GitHubSHA, &github.PullRequestListOptions{ListOptions: github.ListOptions{PerPage: 100}})
		if err != nil {
			return fmt.Errorf("failed to get pull request number for commit sha: %s", g.cfg.GitHubSHA)
		}

		if len(result) == 0 {
			return fmt.Errorf("no pull requests found for commit sha: %s", g.cfg.GitHubSHA)
		}

		g.cfg.GitHubPullRequestNumber = result[0].GetNumber()
	}

	msg, err := entrypointsSummaryMessage(p, g.logURL)
	if err != nil {
		return fmt.Errorf("failed to generate summary message: %w", err)
	}

	if err := g.createIssueComment(
		ctx,
		g.cfg.GitHubOwner,
		g.cfg.GitHubRepo,
		g.cfg.GitHubPullRequestNumber,
		msg.String(),
	); err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// ClearReports clears any existing reports that can be removed.
func (g *GitHub) ClearReports(ctx context.Context) error {
	listOpts := &ListReportsOptions{
		GitHub: &github.IssueListCommentsOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		},
	}

	for {
		response, err := g.ListReports(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Reports == nil {
			return nil
		}

		for _, comment := range response.Reports {
			// prefix is not found, skip
			if !strings.HasPrefix(comment.Body, commentPrefix) {
				continue
			}

			// found the prefix, delete the comment
			if err := g.DeleteReport(ctx, comment.ID); err != nil {
				return fmt.Errorf("failed to delete comment: %w", err)
			}
		}

		if response.Pagination == nil {
			return nil
		}
		listOpts.GitHub.Page = response.Pagination.NextPage
	}
}

// ListReports lists existing comments for an issue or change request.
func (g *GitHub) ListReports(ctx context.Context, opts *ListReportsOptions) (*ListReportsResult, error) {
	var comments []*Report
	var pagination *Pagination

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghComments, resp, err := g.client.Issues.ListComments(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubPullRequestNumber, opts.GitHub)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		for _, c := range ghComments {
			comments = append(comments, &Report{ID: c.GetID(), Body: c.GetBody()})
		}

		if resp.NextPage != 0 {
			pagination = &Pagination{NextPage: resp.NextPage}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return &ListReportsResult{Reports: comments, Pagination: pagination}, nil
}

// DeleteReport deletes an existing comment from an issue or change request.
func (g *GitHub) DeleteReport(ctx context.Context, id int64) error {
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		resp, err := g.client.Issues.DeleteComment(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, id)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to delete pull request comment: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to delete pull request comment: %w", err)
	}

	return nil
}

// createIssueComment creates a comment for an issue or pull request.
func (g *GitHub) createIssueComment(ctx context.Context, owner, repo string, number int, body string) error {
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		_, resp, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to create pull-request/issue comment: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create pull-request/issue comment with retries: %w", err)
	}

	return nil
}

// validateGitHubReporterInputs validates the inputs required for reporting.
func validateGitHubReporterInputs(cfg *gh.Config) error {
	var merr error
	if cfg.GitHubOwner == "" {
		merr = errors.Join(merr, fmt.Errorf("github owner is required"))
	}

	if cfg.GitHubRepo == "" {
		merr = errors.Join(merr, fmt.Errorf("github repo is required"))
	}

	if cfg.GitHubPullRequestNumber <= 0 && cfg.GitHubSHA == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github pull request number or github sha are required"))
	}

	return merr
}

// Job is the GitHub Job that runs as part of a workflow.
type job struct {
	ID   int64
	Name string
	URL  string
}

func (g *GitHub) resolveJobLogsURL(ctx context.Context) (string, error) {
	var jobs []*job

	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghJobs, resp, err := g.client.Actions.ListWorkflowJobs(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, g.cfg.GitHubRunID, nil)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			return fmt.Errorf("failed to list jobs for workflow run attempt: %w", err)
		}

		for _, workflowJob := range ghJobs.Jobs {
			jobs = append(jobs, &job{ID: workflowJob.GetID(), Name: workflowJob.GetName(), URL: *workflowJob.HTMLURL})
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to resolve direct URL to job logs: %w", err)
	}

	for _, job := range jobs {
		if g.cfg.GitHubJobName == job.Name {
			return job.URL, nil
		}
	}
	return "", fmt.Errorf("failed to resolve direct URL to job logs: no job found matching name %s", g.cfg.GitHubJobName)
}

// getPullRequestsForCommit lists the pull requests associated with a commit SHA.
func (g *GitHub) getPullRequestsForCommit(ctx context.Context, sha string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	logger := logging.FromContext(ctx)
	logger.DebugContext(ctx, "looking up pull request from commit sha",
		"owner", g.cfg.GitHubOwner,
		"repo", g.cfg.GitHubRepo,
		"commit_sha", g.cfg.GitHubSHA)

	var ghOpts *github.PullRequestListOptions
	if opts != nil {
		ghOpts = &github.PullRequestListOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}
	}

	pullRequests := make([]*github.PullRequest, 0)
	if err := g.withRetries(ctx, func(ctx context.Context) error {
		ghPullRequests, resp, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, g.cfg.GitHubOwner, g.cfg.GitHubRepo, sha, ghOpts)
		if err != nil {
			if resp != nil {
				if _, ok := ignoredStatusCodes[resp.StatusCode]; !ok {
					return retry.RetryableError(err)
				}
			}
			if resp.StatusCode == http.StatusNotFound {
				return nil
			}
			return fmt.Errorf("failed to list pull request comments: %w", err)
		}

		pullRequests = append(pullRequests, ghPullRequests...)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list pull request comments: %w", err)
	}

	return pullRequests, nil
}
