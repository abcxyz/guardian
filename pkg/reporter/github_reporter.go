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
	"time"

	"github.com/sethvargo/go-githubactions"

	"github.com/abcxyz/guardian/pkg/flags"
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

var githubOperationText = map[Operation]string{
	PlanOperation:    "PLAN",
	ApplyOperation:   "APPLY",
	UnknownOperation: "UNKNOWN",
}

var githubStatusText = map[Status]string{
	StartStatus:     "🟨 STARTED",
	SuccessStatus:   "🟩 SUCCESS",
	NoChangesStatus: "🟦 NO CHANGES",
	FailureStatus:   "🟥 FAILED",
	UnknownStatus:   "⛔️ UNKNOWN",
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
	flags        flags.GitHubFlags
	commentID    int64
	logURL       string
}

// NewGitHubReporter creates a new GitHubReporter.
func NewGitHubReporter(ctx context.Context, f flags.GitHubFlags) (Reporter, error) {
	logger := logging.FromContext(ctx)

	ghCtx, err := githubactions.New().Context()
	if err != nil {
		return nil, fmt.Errorf("failed to create github context: %w", err)
	}

	f.FromGitHubContext(ghCtx)

	if err := validateFlags(f); err != nil {
		return nil, fmt.Errorf("failed to validate github flags: %w", err)
	}

	tokenSource, err := f.TokenSource(ctx, map[string]string{
		"contents":      "read",
		"pull_requests": "write",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}
	token, err := tokenSource.GitHubToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	gc := github.NewClient(
		ctx,
		token,
		github.WithRetryInitialDelay(1*time.Second),
		github.WithRetryMaxAttempts(3),
		github.WithRetryMaxDelay(30*time.Second),
	)

	if f.FlagGitHubPullRequestNumber <= 0 {
		prResponse, err := gc.ListPullRequestsForCommit(ctx, f.FlagGitHubOwner, f.FlagGitHubRepo, f.FlagGitHubSHA, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get pull request number for commit sha: %w", err)
		}

		if len(prResponse.PullRequests) == 0 {
			return nil, fmt.Errorf("no pull requests found for commit sha: %s", f.FlagGitHubSHA)
		}

		f.FlagGitHubPullRequestNumber = prResponse.PullRequests[0].Number
	}

	logger.DebugContext(ctx, "computed pull request number", "computed_pull_request_number", f.FlagGitHubPullRequestNumber)

	logURL, err := gc.ResolveJobLogsURL(ctx, f.FlagGitHubJob, f.FlagGitHubOwner, f.FlagGitHubRepo, f.FlagGitHubRunID)
	if err != nil {
		logger.WarnContext(ctx, "could not resolve direct url to job logs", "err", err)
		logURL = fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", f.FlagGitHubServerURL, f.FlagGitHubOwner, f.FlagGitHubRepo, f.FlagGitHubRunID, f.FlagGitHubRunAttempt)
	}

	return &GitHubReporter{
		gitHubClient: gc,
		flags:        f,
		logURL:       logURL,
	}, nil
}

// validateFlags validates the required GitHub flags.
func validateFlags(f flags.GitHubFlags) error {
	var merr error

	if f.FlagGitHubToken == "" && f.FlagGitHubAppID == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github token or github app id are required"))
	}

	if f.FlagGitHubToken != "" && f.FlagGitHubAppID != "" {
		merr = errors.Join(merr, fmt.Errorf("only one of github token or github app id are allowed"))
	}

	if f.FlagGitHubAppID != "" && f.FlagGitHubAppInstallationID == "" {
		merr = errors.Join(merr, fmt.Errorf("a github app installation id is required when using a github app id"))
	}

	if f.FlagGitHubAppID != "" && f.FlagGitHubAppPrivateKeyPEM == "" {
		merr = errors.Join(merr, fmt.Errorf("a github app private key is required when using a github app id"))
	}

	if f.FlagGitHubOwner == "" {
		merr = errors.Join(merr, fmt.Errorf("github owner is required"))
	}

	if f.FlagGitHubRepo == "" {
		merr = errors.Join(merr, fmt.Errorf("github repo is required"))
	}

	if f.FlagGitHubPullRequestNumber <= 0 && f.FlagGitHubSHA == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github pull request number or github sha are required"))
	}

	if f.FlagGitHubServerURL == "" {
		merr = errors.Join(merr, fmt.Errorf("github server url is required"))
	}

	if f.FlagGitHubRunID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("github run id is required"))
	}

	if f.FlagGitHubRunAttempt <= 0 {
		merr = errors.Join(merr, fmt.Errorf("github run attempt is required"))
	}

	return merr
}

// CreateStatus implements the reporter Status function. It creates a new GitHub comment and persists the comment ID
// in a private struct var for use by the UpdateStatus function.
func (g *GitHubReporter) CreateStatus(ctx context.Context, p *Params) error {
	msg, err := g.statusMessage(p)
	if err != nil {
		return fmt.Errorf("failed to generate status message")
	}

	startComment, err := g.gitHubClient.CreateIssueComment(
		ctx,
		g.flags.FlagGitHubOwner,
		g.flags.FlagGitHubRepo,
		g.flags.FlagGitHubPullRequestNumber,
		msg.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	g.commentID = startComment.ID

	return nil
}

// UpdateStatus implements the reporter UpdateStatus function. It updates an existing GitHub comment text using the comment ID
// persisted from the CreateStatus function.
func (g *GitHubReporter) UpdateStatus(ctx context.Context, p *Params) error {
	if g.commentID == 0 {
		return fmt.Errorf("GitHub comment ID is required to report success")
	}

	msg, err := g.statusMessage(p)
	if err != nil {
		return fmt.Errorf("failed to generate status message")
	}

	if err := g.gitHubClient.UpdateIssueComment(
		ctx,
		g.flags.FlagGitHubOwner,
		g.flags.FlagGitHubRepo,
		g.commentID,
		msg.String(),
	); err != nil {
		return fmt.Errorf("failed to report: %w", err)
	}

	return nil
}

// statusMessage generates the status message based on the provided reporter values.
func (g *GitHubReporter) statusMessage(p *Params) (strings.Builder, error) {
	var msg strings.Builder

	if _, err := msg.WriteString(githubCommentPrefix); err != nil {
		return msg, fmt.Errorf("failed to write Guardian prefix to report: %w", err)
	}

	operationText, ok := githubOperationText[p.Operation]
	if !ok {
		operationText = githubOperationText[UnknownOperation]
	}

	if _, err := msg.WriteString(fmt.Sprintf(" %s", g.markdownPill(operationText))); err != nil {
		return msg, fmt.Errorf("failed to write Guardian operation indicator to report: %w", err)
	}

	if p.IsDestroy {
		if _, err := msg.WriteString(fmt.Sprintf(" %s", g.markdownPill(githubDestroyIndicatorText))); err != nil {
			return msg, fmt.Errorf("failed to write destroy indicator to report: %w", err)
		}
	}

	statusText, ok := githubStatusText[p.Status]
	if !ok {
		statusText = githubStatusText[UnknownStatus]
	}

	if _, err := msg.WriteString(fmt.Sprintf(" %s", g.markdownPill(statusText))); err != nil {
		return msg, fmt.Errorf("failed to write status indicator to report: %w", err)
	}

	if g.logURL != "" {
		if _, err := msg.WriteString(fmt.Sprintf(" [%s]", g.markdownURL("logs", g.logURL))); err != nil {
			return msg, fmt.Errorf("failed to write log url to report: %w", err)
		}
	}

	if p.EntrypointDir != "" {
		if _, err := msg.WriteString(fmt.Sprintf("\n\n**Entrypoint:** %s", p.EntrypointDir)); err != nil {
			return msg, fmt.Errorf("failed to write entrypointDir to report: %w", err)
		}
	}

	detailsText := ""
	if strings.TrimSpace(p.Output) != "" {
		formattedOutput := g.formatOutputForGitHubDiff(p.Output)
		detailsText = fmt.Sprintf("\n\n%s", g.markdownZippy("Output", formattedOutput))
	}

	// if the length of the entire message would exceed the max length, add a truncated message.
	totalLength := len([]rune(msg.String())) + len([]rune(detailsText))
	if totalLength > githubMaxCommentLength {
		if _, err := msg.WriteString(githubTruncatedMessage); err != nil {
			return msg, fmt.Errorf("failed to write truncated message to report: %w", err)
		}
		return msg, nil
	}

	if _, err := msg.WriteString(detailsText); err != nil {
		return msg, fmt.Errorf("failed to write output to report: %w", err)
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

// formatOutputForGitHubDiff formats the Terraform diff output for use with
// GitHub diff markdown formatting.
func (g *GitHubReporter) formatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
