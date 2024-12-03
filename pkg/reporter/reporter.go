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

// Package reporter provides an SDK for reporting Guardian results.
package reporter

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/platform/config"
	gitlab "github.com/xanzy/go-gitlab"
)

const (
	TypeNone   string = "none"
	TypeGitHub string = "github"
	TypeGitLab string = "gitlab"
	TypeFile   string = "file"
)

// SortedReporterTypes are the sorted Reporter types for printing messages and prediction.
var SortedReporterTypes = func() []string {
	allowed := append([]string{}, TypeNone, TypeGitHub, TypeGitLab)
	sort.Strings(allowed)
	return allowed
}()

// Status is the result of the operation Guardian is performing.
type Status string

// the supported statuses for reporters.
const (
	StatusSuccess         Status = Status("SUCCESS")          //nolint:errname // Not an error
	StatusFailure         Status = Status("FAILURE")          //nolint:errname // Not an error
	StatusNoOperation     Status = Status("NO CHANGES")       //nolint:errname // Not an error
	StatusPolicyViolation Status = Status("POLICY VIOLATION") //nolint:errname // Not an error
	StatusUnknown         Status = Status("UNKNOWN")          //nolint:errname // Not an error
)

var statusText = map[Status]string{
	StatusSuccess:         "🟩 SUCCESS",
	StatusNoOperation:     "🟦 NO CHANGES",
	StatusFailure:         "🟥 FAILED",
	StatusUnknown:         "⛔️ UNKNOWN",
	StatusPolicyViolation: "🚨 ATTENTION REQUIRED",
}

// StatusParams are the parameters for writing status reports.
type StatusParams struct {
	HasDiff   bool
	Details   string
	Dir       string
	Message   string
	Operation string
}

// EntrypointsSummaryParams are the parameters for writing entrypoints summary reports.
type EntrypointsSummaryParams struct {
	Message string
	Dirs    []string
}

// Reporter defines the minimum interface for a reporter.
type Reporter interface {
	// Status reports the status of a run.
	Status(ctx context.Context, status Status, params *StatusParams) error

	// EntrypointsSummary reports the summary for the entrypionts command.
	EntrypointsSummary(ctx context.Context, params *EntrypointsSummaryParams) error

	// Clear clears any existing reports that can be removed.
	Clear(ctx context.Context) error
}

// Config is the configuration needed to generate different reporter types.
type Config struct {
	GitHub github.Config
	GitLab config.GitLab
}

// NewReporter creates a new reporter based on the provided type.
func NewReporter(ctx context.Context, t string, c *Config) (Reporter, error) {
	if strings.EqualFold(t, TypeNone) {
		return NewNoopReporter(ctx)
	}

	if strings.EqualFold(t, TypeFile) {
		return NewFileReporter()
	}

	if strings.EqualFold(t, TypeGitHub) {
		c.GitHub.Permissions = map[string]string{
			"contents":      "read",
			"pull_requests": "write",
		}

		gc, err := github.NewGitHubClient(ctx, &c.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}

		return NewGitHubReporter(ctx, gc, &GitHubReporterInputs{
			GitHubOwner:             c.GitHub.GitHubOwner,
			GitHubRepo:              c.GitHub.GitHubRepo,
			GitHubPullRequestNumber: c.GitHub.GitHubPullRequestNumber,
			GitHubServerURL:         c.GitHub.GitHubServerURL,
			GitHubRunID:             c.GitHub.GitHubRunID,
			GitHubRunAttempt:        c.GitHub.GitHubRunAttempt,
			GitHubJob:               c.GitHub.GitHubJob,
			GitHubJobName:           c.GitHub.GitHubJobName,
			GitHubSHA:               c.GitHub.GitHubSHA,
		})
	}

	if strings.EqualFold(t, TypeGitLab) {
		gc, err := gitlab.NewClient(c.GitLab.GitLabToken, gitlab.WithBaseURL(c.GitLab.GitLabBaseURL))
		if err != nil {
			return nil, fmt.Errorf("failed to create gitlab client: %w", err)
		}

		return NewGitLabReporter(ctx, gc, &GitLabReporterInputs{
			GitLabProjectID:      c.GitLab.GitLabProjectID,
			GitLabMergeRequestID: c.GitLab.GitLabMergeRequestID,
		})
	}
	return nil, fmt.Errorf("unknown reporter type: %s", t)
}

// markdownPill returns a markdown element that is bolded and wraped in a inline code block.
func markdownPill(text string) string {
	return fmt.Sprintf("**`%s`**", text)
}

// markdownURL returns a markdown URL string given a title and a URL.
func markdownURL(text, URL string) string {
	return fmt.Sprintf("[%s](%s)", text, URL)
}

// markdonZippy returns a collapsible section with a given title and body.
func markdownZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n</details>", title, body)
}

// markdonDiffZippy returns a collapsible section with a given title and body.
func markdownDiffZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```diff\n\n%s\n```\n</details>", title, body)
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

// formatOutputForDiff formats the Terraform diff output for use with
// diff markdown formatting.
func formatOutputForDiff(content string) string {
	content = tildeChanged.ReplaceAllString(content, `$1!`)
	content = swapLeadingWhitespace.ReplaceAllString(content, "$2$1")

	return content
}

const (
	commentPrefix    = "#### 🔱 Guardian 🔱"
	truncatedMessage = "\n\n> Message has been truncated. See workflow logs to view the full message."
)

// statusMessage generates the status message based on the provided reporter values.
func statusMessage(st Status, p *StatusParams, logURL string, maxCommentLength int) (strings.Builder, error) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "%s", commentPrefix)

	operationText := strings.ToUpper(strings.TrimSpace(p.Operation))
	if operationText != "" {
		fmt.Fprintf(&msg, " %s", markdownPill(operationText))
	}

	stText, ok := statusText[st]
	if !ok {
		stText = statusText[StatusUnknown]
	}

	fmt.Fprintf(&msg, " %s", markdownPill(stText))

	if logURL != "" {
		fmt.Fprintf(&msg, " [%s]", markdownURL("logs", logURL))
	}

	if p.Dir != "" {
		fmt.Fprintf(&msg, "\n\n**Entrypoint:** %s", p.Dir)
	}

	if p.Message != "" {
		fmt.Fprintf(&msg, "\n\n %s", p.Message)
	}

	if p.Details != "" {
		detailsText := fmt.Sprintf("\n\n%s", markdownZippy("Details", p.Details))

		if p.HasDiff {
			detailsText = fmt.Sprintf("\n\n%s", markdownDiffZippy("Details", formatOutputForDiff(p.Details)))
		}

		// if the length of the entire message would exceed the max length
		// append a truncated message instead of the details text.
		totalLength := len([]rune(msg.String())) + len([]rune(detailsText))
		if maxCommentLength >= 0 && totalLength > maxCommentLength {
			detailsText = truncatedMessage
		}

		fmt.Fprintf(&msg, "%s", detailsText)
	}

	return msg, nil
}

// entrypointsSummaryMessage generates the entrypoints summary message based on the provided reporter values.
func entrypointsSummaryMessage(p *EntrypointsSummaryParams, logURL string) (strings.Builder, error) {
	var msg strings.Builder

	fmt.Fprintf(&msg, "%s", commentPrefix)

	if logURL != "" {
		fmt.Fprintf(&msg, " [%s]", markdownURL("logs", logURL))
	}

	if p.Message != "" {
		fmt.Fprintf(&msg, "\n\n%s", p.Message)
	}

	if len(p.Dirs) > 0 {
		fmt.Fprintf(&msg, "\n\n**%s**\n%s", "Directories", strings.Join(p.Dirs, "\n"))
	}

	return msg, nil
}
