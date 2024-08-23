// Copyright 2024 The Authors (see AUTHORS file)
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

package reporter

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	gh "github.com/google/go-github/v53/github"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/logging"
)

var _ Reporter = (*GitHubReporter)(nil)

const (
	githubCommentPrefix        = "#### 🔱 Guardian 🔱"
	githubMaxCommentLength     = 65536
	githubDestroyIndicatorText = "💥 DESTROY"
	githubTruncatedMessage     = "\n\n> Message has been truncated. See workflow logs to view the full message."
)

var githubStatusText = map[Status]string{
	StatusSuccess:     "🟩 SUCCESS",
	StatusNoOperation: "🟦 NO CHANGES",
	StatusFailure:     "🟥 FAILED",
	StatusUnknown:     "⛔️ UNKNOWN",
}

var (
	tildeChanged = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			"([~])") // tilde represents changes and needs switched to exclamation for git diff

	swapLeadingWhitespace = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			`((\-(\/\+)*)|(\+(\/\-)*)|(!))`) // match characters to swap whitespace for git diff (+, +/-, -, -/+, !)
)

// GitHubReporter implements the reporter interface for writing GitHub comments.
type GitHubReporter struct {
	gitHubClient github.GitHub
	inputs       *GitHubReporterInputs
	logURL       string
}

// GitHubReporterInputs are the inputs used by the GitHub reporter.
type GitHubReporterInputs struct {
	GitHubOwner             string
	GitHubRepo              string
	GitHubServerURL         string
	GitHubRunID             int64
	GitHubRunAttempt        int64
	GitHubJob               string
	GitHubJobName           string
	GitHubPullRequestNumber int
	GitHubSHA               string
}

// Validate validates the required inputs.
func (i *GitHubReporterInputs) Validate() error {
	var merr error
	if i.GitHubOwner == "" {
		merr = errors.Join(merr, fmt.Errorf("github owner is required"))
	}

	if i.GitHubRepo == "" {
		merr = errors.Join(merr, fmt.Errorf("github repo is required"))
	}

	if i.GitHubPullRequestNumber <= 0 && i.GitHubSHA == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github pull request number or github sha are required"))
	}

	return merr
}

// NewGitHubReporter creates a new GitHubReporter.
func NewGitHubReporter(ctx context.Context, gc github.GitHub, i *GitHubReporterInputs) (Reporter, error) {
	logger := logging.FromContext(ctx)

	if gc == nil {
		return nil, fmt.Errorf("github client is required")
	}

	if err := i.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate github reporter inputs: %w", err)
	}

	if i.GitHubPullRequestNumber <= 0 {
		prResponse, err := gc.ListPullRequestsForCommit(ctx, i.GitHubOwner, i.GitHubRepo, i.GitHubSHA, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get pull request number for commit sha: %w", err)
		}

		if len(prResponse.PullRequests) == 0 {
			return nil, fmt.Errorf("no pull requests found for commit sha: %s", i.GitHubSHA)
		}

		i.GitHubPullRequestNumber = prResponse.PullRequests[0].Number
	}

	logger.DebugContext(ctx, "computed pull request number", "computed_pull_request_number", i.GitHubPullRequestNumber)

	r := &GitHubReporter{
		gitHubClient: gc,
		inputs:       i,
	}

	var logURL string
	if i.GitHubServerURL != "" || i.GitHubRunID > 0 || i.GitHubRunAttempt > 0 {
		logURL = fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", i.GitHubServerURL, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID, i.GitHubRunAttempt)
	}

	if i.GitHubJobName != "" {
		resolvedURL, err := gc.ResolveJobLogsURL(ctx, i.GitHubJobName, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID)
		if err != nil {
			resolvedURL = logURL
		}
		r.logURL = resolvedURL
	}

	return r, nil
}

// Status implements the reporter Status function by writing a GitHub comment.
func (g *GitHubReporter) Status(ctx context.Context, st Status, p *StatusParams) error {
	msg, err := g.statusMessage(st, p)
	if err != nil {
		return fmt.Errorf("failed to generate status message: %w", err)
	}

	_, err = g.gitHubClient.CreateIssueComment(
		ctx,
		g.inputs.GitHubOwner,
		g.inputs.GitHubRepo,
		g.inputs.GitHubPullRequestNumber,
		msg.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// EntrypointsSummary implements the reporter EntrypointsSummary function by writing a GitHub comment.
func (g *GitHubReporter) EntrypointsSummary(ctx context.Context, p *EntrypointsSummaryParams) error {
	msg, err := g.entrypointsSummaryMessage(p)
	if err != nil {
		return fmt.Errorf("failed to generate summary message: %w", err)
	}

	if _, err = g.gitHubClient.CreateIssueComment(
		ctx,
		g.inputs.GitHubOwner,
		g.inputs.GitHubRepo,
		g.inputs.GitHubPullRequestNumber,
		msg.String(),
	); err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// Clear deletes all github comments with the guardian prefix.
func (g *GitHubReporter) Clear(ctx context.Context) error {
	listOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	for {
		response, err := g.gitHubClient.ListIssueComments(ctx, g.inputs.GitHubOwner, g.inputs.GitHubRepo, g.inputs.GitHubPullRequestNumber, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		if response.Comments == nil {
			return nil
		}

		for _, comment := range response.Comments {
			// prefix is not found, skip
			if !strings.HasPrefix(comment.Body, githubCommentPrefix) {
				continue
			}

			// found the prefix, delete the comment
			if err := g.gitHubClient.DeleteIssueComment(ctx, g.inputs.GitHubOwner, g.inputs.GitHubRepo, comment.ID); err != nil {
				return fmt.Errorf("failed to delete comment: %w", err)
			}
		}

		if response.Pagination == nil {
			return nil
		}
		listOpts.Page = response.Pagination.NextPage
	}
}

// statusMessage generates the status message based on the provided reporter values.
func (g *GitHubReporter) statusMessage(st Status, p *StatusParams) (strings.Builder, error) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "%s", githubCommentPrefix)

	operationText := strings.ToUpper(strings.TrimSpace(p.Operation))
	if operationText != "" {
		fmt.Fprintf(&msg, " %s", g.markdownPill(operationText))
	}

	if p.IsDestroy {
		fmt.Fprintf(&msg, " %s", g.markdownPill(githubDestroyIndicatorText))
	}

	statusText, ok := githubStatusText[st]
	if !ok {
		statusText = githubStatusText[StatusUnknown]
	}

	fmt.Fprintf(&msg, " %s", g.markdownPill(statusText))

	if g.logURL != "" {
		fmt.Fprintf(&msg, " [%s]", g.markdownURL("logs", g.logURL))
	}

	if p.Dir != "" {
		fmt.Fprintf(&msg, "\n\n**Entrypoint:** %s", p.Dir)
	}

	if p.Message != "" {
		fmt.Fprintf(&msg, "\n\n %s", p.Message)
	}

	if p.Details != "" {
		detailsText := fmt.Sprintf("\n\n%s", g.markdownZippy("Details", p.Details))

		if p.HasDiff {
			detailsText = fmt.Sprintf("\n\n%s", g.markdownDiffZippy("Details", formatOutputForGitHubDiff(p.Details)))
		}

		// if the length of the entire message would exceed the max length
		// append a truncated message instead of the details text.
		totalLength := len([]rune(msg.String())) + len([]rune(detailsText))
		if totalLength > githubMaxCommentLength {
			detailsText = githubTruncatedMessage
		}

		fmt.Fprintf(&msg, "%s", detailsText)
	}

	return msg, nil
}

// entrypointsSummaryMessage generates the entrypoints summary message based on the provided reporter values.
func (g *GitHubReporter) entrypointsSummaryMessage(p *EntrypointsSummaryParams) (strings.Builder, error) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "%s", githubCommentPrefix)

	if g.logURL != "" {
		fmt.Fprintf(&msg, " [%s]", g.markdownURL("logs", g.logURL))
	}

	if p.Message != "" {
		fmt.Fprintf(&msg, "\n\n%s\n", p.Message)
	}

	if len(p.ModifiedDirs) > 0 {
		fmt.Fprintf(&msg, "\n**%s**\n%s", "Plan", strings.Join(p.ModifiedDirs, "\n"))
	}

	if len(p.DestroyDirs) > 0 {
		fmt.Fprintf(&msg, "\n**%s**\n%s", "Destroy", strings.Join(p.DestroyDirs, "\n"))
	}

	if len(p.AbandonedDirs) > 0 {
		fmt.Fprintf(&msg, "\n**%s**\n%s", "Abandon", strings.Join(p.AbandonedDirs, "\n"))
	}

	return msg, nil
}

// markdownPill returns a markdown element that is bolded and wraped in a code block.
func (g *GitHubReporter) markdownPill(text string) string {
	return fmt.Sprintf("**`%s`**", text)
}

// markdownURL returns a markdown URL string given a title and a URL.
func (g *GitHubReporter) markdownURL(text, URL string) string {
	return fmt.Sprintf("[%s](%s)", text, URL)
}

// markdonZippy returns a collapsible section with a given title and body.
func (g *GitHubReporter) markdownZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```\n\n%s\n```\n</details>", title, body)
}

// markdonDiffZippy returns a collapsible section with a given title and body.
func (g *GitHubReporter) markdownDiffZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```diff\n\n%s\n```\n</details>", title, body)
}

// formatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func formatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
