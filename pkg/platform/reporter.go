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

package platform

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	StatusSuccess         Status = Status("SUCCESS")
	StatusFailure         Status = Status("FAILURE")
	StatusNoOperation     Status = Status("NO CHANGES")
	StatusPolicyViolation Status = Status("POLICY VIOLATION")
	StatusUnknown         Status = Status("UNKNOWN")
)

var (
	tildeChanged = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			"([~])") // tilde represents changes and needs switched to exclamation for git diff

	swapLeadingWhitespace = regexp.MustCompile(
		"(?m)" + // enable multi-line mode
			"^([\t ]*)" + // only match tilde at start of line, can lead with tabs or spaces
			`((\-(\/\+)*)|(\+(\/\-)*)|(!))`) // match characters to swap whitespace for git diff (+, +/-, -, -/+, !)

	statusText = map[Status]string{
		StatusSuccess:         "🟩 SUCCESS",
		StatusNoOperation:     "🟦 NO CHANGES",
		StatusFailure:         "🟥 FAILED",
		StatusUnknown:         "⛔️ UNKNOWN",
		StatusPolicyViolation: "🚨 ATTENTION REQUIRED",
	}
)

// Status is the result of the operation Guardian is performing.
type Status string

// StatusParams are the parameters for writing status reports.
type StatusParams struct {
	HasDiff      bool
	Details      string
	Dir          string
	ErrorMessage string
	Message      string
	Operation    string
}

// EntrypointsSummaryParams are the parameters for writing entrypoints summary reports.
type EntrypointsSummaryParams struct {
	Message string
	Dirs    []string
}

// markdownPill returns a markdown element that is bolded and wraped in a inline code block.
func markdownPill(text string) string {
	return fmt.Sprintf("**`%s`**", text)
}

// markdownURL returns a markdown URL string given a title and a URL.
func markdownURL(text, URL string) string {
	return fmt.Sprintf("[%s](%s)", text, URL)
}

// markdownZippy returns a collapsible section with a given title and body.
func markdownZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n`%s`\n</details>", title, body)
}

// markdonDiffZippy returns a collapsible section with a given title and body.
func markdownDiffZippy(title, body string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n```diff\n\n%s\n```\n</details>", title, body)
}

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

	if p.ErrorMessage != "" {
		fmt.Fprintf(&msg, "\n\n **Error:** `%s`", p.ErrorMessage)
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
