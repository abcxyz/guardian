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

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/githubauth"
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
	GitHubToken             string
	GitHubOwner             string
	GitHubRepo              string
	GitHubAppID             string
	GitHubAppInstallationID string
	GitHubAppPrivateKeyPEM  string
	GitHubServerURL         string
	GitHubRunID             int64
	GitHubRunAttempt        int64
	GitHubJob               string
	GitHubPullRequestNumber int
	GitHubSHA               string
}

// Validate validates the required inputs.
func (i *GitHubReporterInputs) Validate() error {
	var merr error

	if i.GitHubToken == "" && i.GitHubAppID == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github token or github app id are required"))
	}

	if i.GitHubToken != "" && i.GitHubAppID != "" {
		merr = errors.Join(merr, fmt.Errorf("only one of github token or github app id are allowed"))
	}

	if i.GitHubAppID != "" && i.GitHubAppInstallationID == "" {
		merr = errors.Join(merr, fmt.Errorf("a github app installation id is required when using a github app id"))
	}

	if i.GitHubAppID != "" && i.GitHubAppPrivateKeyPEM == "" {
		merr = errors.Join(merr, fmt.Errorf("a github app private key is required when using a github app id"))
	}

	if i.GitHubOwner == "" {
		merr = errors.Join(merr, fmt.Errorf("github owner is required"))
	}

	if i.GitHubRepo == "" {
		merr = errors.Join(merr, fmt.Errorf("github repo is required"))
	}

	if i.GitHubPullRequestNumber <= 0 && i.GitHubSHA == "" {
		merr = errors.Join(merr, fmt.Errorf("one of github pull request number or github sha are required"))
	}

	if i.GitHubServerURL == "" {
		merr = errors.Join(merr, fmt.Errorf("github server url is required"))
	}

	if i.GitHubRunID <= 0 {
		merr = errors.Join(merr, fmt.Errorf("github run id is required"))
	}

	if i.GitHubRunAttempt <= 0 {
		merr = errors.Join(merr, fmt.Errorf("github run attempt is required"))
	}

	return merr
}

// NewGitHubReporter creates a new GitHubReporter.
func NewGitHubReporter(ctx context.Context, i *GitHubReporterInputs) (Reporter, error) {
	logger := logging.FromContext(ctx)

	if err := i.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate github reporter inputs: %w", err)
	}

	tokenSource, err := TokenSource(ctx, i, map[string]string{
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

	logURL, err := gc.ResolveJobLogsURL(ctx, i.GitHubJob, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID)
	if err != nil {
		logger.WarnContext(ctx, "could not resolve direct url to job logs", "err", err)
		logURL = fmt.Sprintf("%s/%s/%s/actions/runs/%d/attempts/%d", i.GitHubServerURL, i.GitHubOwner, i.GitHubRepo, i.GitHubRunID, i.GitHubRunAttempt)
	}

	return &GitHubReporter{
		gitHubClient: gc,
		inputs:       i,
		logURL:       logURL,
	}, nil
}

// TokenSource creates a token source for a github client to call the GitHub API.
func TokenSource(ctx context.Context, i *GitHubReporterInputs, permissions map[string]string) (githubauth.TokenSource, error) {
	if i.GitHubToken != "" {
		githubTokenSource, err := githubauth.NewStaticTokenSource(i.GitHubToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create github static token source: %w", err)
		}
		return githubTokenSource, nil
	} else {
		app, err := githubauth.NewApp(
			i.GitHubAppID,
			i.GitHubAppPrivateKeyPEM,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app token source: %w", err)
		}

		installation, err := app.InstallationForID(ctx, i.GitHubAppInstallationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get github app installation: %w", err)
		}

		return installation.SelectedReposTokenSource(permissions, i.GitHubRepo), nil
	}
}

// CreateStatus implements the reporter Status function by writing a GitHub status comment.
func (g *GitHubReporter) CreateStatus(ctx context.Context, st Status, p *Params) error {
	msg, err := g.statusMessage(st, p)
	if err != nil {
		return fmt.Errorf("failed to generate status message")
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

// statusMessage generates the status message based on the provided reporter values.
func (g *GitHubReporter) statusMessage(st Status, p *Params) (strings.Builder, error) {
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

	if p.Output != "" {
		detailsText := fmt.Sprintf("\n\n%s", g.markdownZippy("Output", p.Output))

		if p.HasDiff {
			detailsText = fmt.Sprintf("\n\n%s", g.markdownDiffZippy("Output", g.formatOutputForGitHubDiff(p.Output)))
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
func (g *GitHubReporter) formatOutputForGitHubDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}
