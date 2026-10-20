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

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/google/go-github/v53/github"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/pkg/plansummary"
	"github.com/abcxyz/guardian/pkg/platform"
	"github.com/abcxyz/pkg/cli"
)

var _ cli.Command = (*ReportCommand)(nil)

type ReportCommand struct {
	cli.BaseCommand

	platformConfig platform.Config

	flagType         string
	flagEntrypoints  string
	flagArtifactsDir string

	platformClient platform.Platform
}

func (c *ReportCommand) Desc() string {
	return `Aggregate and report Guardian plan/apply status on pull requests`
}

func (c *ReportCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

	Aggregate and report Guardian plan/apply status on pull requests.
`
}

func (c *ReportCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.platformConfig.RegisterFlags(set)

	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "type",
		Target:  &c.flagType,
		Example: "plan",
		Usage:   "The type of the report, either 'plan' or 'apply'.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "entrypoints",
		Target:  &c.flagEntrypoints,
		Example: `["terraform/github/abseil"]`,
		Usage:   "The list of directory entrypoints as a JSON array string.",
	})

	f.StringVar(&cli.StringVar{
		Name:    "artifacts-dir",
		Target:  &c.flagArtifactsDir,
		Example: "./artifacts",
		Usage:   fmt.Sprintf("The local path where %s artifacts are downloaded (required for plan type).", plansummary.Filename),
	})

	set.AfterParse(func(existingErr error) (merr error) {
		if c.flagType != "plan" && c.flagType != "apply" {
			merr = errors.Join(merr, fmt.Errorf("missing or invalid flag: type must be 'plan' or 'apply'"))
		}

		if c.flagEntrypoints == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: entrypoints is required"))
		}

		if c.flagType == "plan" && c.flagArtifactsDir == "" {
			merr = errors.Join(merr, fmt.Errorf("missing flag: artifacts-dir is required when type is 'plan'"))
		}

		return merr
	})

	return set
}

func (c *ReportCommand) Run(ctx context.Context, args []string) error {
	metricswrap.WriteMetric(ctx, "command_workflows_report", 1)

	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()
	if len(parsedArgs) > 0 {
		return flag.ErrHelp
	}

	platform, err := platform.NewPlatform(ctx, &c.platformConfig)
	if err != nil {
		return fmt.Errorf("failed to create platform: %w", err)
	}
	c.platformClient = platform

	return c.Process(ctx)
}

func (c *ReportCommand) Process(ctx context.Context) error {
	var prNumber int
	if c.flagType == "plan" {
		prNumber = c.platformConfig.GitHub.GitHubPullRequestNumber
	} else {
		// For apply (on push event), look up PR number associated with the commit SHA
		resp, err := c.platformClient.ListChangeRequestsByCommit(ctx, c.platformConfig.GitHub.GitHubSHA, nil)
		if err != nil {
			return fmt.Errorf("failed to list pull requests for commit [%s]: %w", c.platformConfig.GitHub.GitHubSHA, err)
		}
		if len(resp.PullRequests) == 0 {
			c.Outf("no pull requests found for commit [%s], skipping reporting", c.platformConfig.GitHub.GitHubSHA)
			return nil
		}
		prNumber = resp.PullRequests[0].Number
	}

	// Fetch all Jobs for current run ID
	jobs, err := c.platformClient.ListJobs(ctx, c.platformConfig.GitHub.GitHubRunID)
	if err != nil {
		return fmt.Errorf("failed to list jobs for run [%d]: %w", c.platformConfig.GitHub.GitHubRunID, err)
	}

	// Parse entrypoints
	var entrypoints []string
	var entrypointsData []byte
	if _, err := os.Stat(c.flagEntrypoints); err == nil {
		entrypointsData, err = os.ReadFile(c.flagEntrypoints)
		if err != nil {
			return fmt.Errorf("failed to read entrypoints file [%s]: %w", c.flagEntrypoints, err)
		}
	} else {
		entrypointsData = []byte(c.flagEntrypoints)
	}

	if err := json.Unmarshal(entrypointsData, &entrypoints); err != nil {
		return fmt.Errorf("failed to parse entrypoints JSON: %w", err)
	}

	// Map entrypoints to GHA jobs
	jobStatuses := make(map[string]*platform.Job)
	for _, entrypoint := range entrypoints {
		parts := strings.Split(entrypoint, "/")
		var candidates []string
		for _, part := range parts {
			if part != "" {
				candidates = append(candidates, part)
			}
		}

		for _, job := range jobs {
			lowerJobName := strings.ToLower(job.Name)
			lowerEntrypoint := strings.ToLower(entrypoint)

			// Works for standard matrix where job name contains full entrypoint path
			if strings.Contains(lowerJobName, lowerEntrypoint) {
				jobStatuses[entrypoint] = job
				break
			}

			// Works for org batching where job name contains org name in parentheses
			matched := false
			for _, candidate := range candidates {
				if candidate != "" && strings.Contains(lowerJobName, "("+strings.ToLower(candidate)+")") {
					jobStatuses[entrypoint] = job
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}

	var summaries map[string]*plansummary.Summary
	if c.flagType == "plan" {
		summaries, err = loadPlanSummaries(c.flagArtifactsDir)
		if err != nil {
			return err
		}
	}

	// Generate Markdown Summary Comment
	rows := make([]summaryRow, 0, len(entrypoints))
	for _, entrypoint := range entrypoints {
		status := "⛔&nbsp;UNKNOWN"
		logLink := "-"
		notes := "-"
		statsStr := "-"

		job, hasJob := jobStatuses[entrypoint]
		if hasJob {
			logLink = fmt.Sprintf("<a href=\"%s\" target=\"_blank\">View Log</a>", job.URL)
			switch job.Conclusion {
			case "success":
				status = "🟩&nbsp;SUCCESS"
			case "failure":
				status = "🟥&nbsp;FAILED"
			case "skipped":
				status = "🟨&nbsp;SKIPPED"
			case "cancelled":
				status = "🟨&nbsp;CANCELLED"
			default:
				status = fmt.Sprintf("⛔&nbsp;%s", strings.ToUpper(job.Conclusion))
			}
		} else {
			notes = "⚠️ Job not found in run"
		}

		if c.flagType == "plan" {
			summary, ok := summaries[filepath.ToSlash(filepath.Clean(entrypoint))]
			if ok {
				statsStr = fmt.Sprintf("<span style=\"white-space: nowrap;\">%+d&nbsp;~%d&nbsp;-%d</span>", summary.Added, summary.Changed, summary.Deleted)
				if hasJob {
					notes = "-"
				}
			} else if hasJob {
				if job.Conclusion == "skipped" || job.Conclusion == "cancelled" {
					notes = "-"
				} else {
					notes = "⚠️ Missing plan summary"
				}
			} else {
				notes = "⚠️ Job not found in run"
			}
		}
		rows = append(rows, summaryRow{
			Directory: entrypoint,
			Status:    status,
			Stats:     statsStr,
			Notes:     notes,
			LogLink:   logLink,
		})
	}

	const maxLength = 60000
	var commentPrefix string
	var tmplText string
	if c.flagType == "plan" {
		commentPrefix = "#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**"
		tmplText = planSummaryTemplate
	} else {
		commentPrefix = "#### 🔱 Guardian 🔱 **`APPLY SUMMARY`**"
		tmplText = applySummaryTemplate
	}

	currentLen := len(commentPrefix) + 200
	finalRows := make([]summaryRow, 0, len(rows))
	var truncated bool
	for _, r := range rows {
		rowLen := len(r.Directory) + len(r.Status) + len(r.Stats) + len(r.Notes) + len(r.LogLink) + 20
		if currentLen+rowLen > maxLength {
			truncated = true
			break
		}
		finalRows = append(finalRows, r)
		currentLen += rowLen
	}

	var sb strings.Builder
	tmpl, err := template.New("summary").Parse(tmplText)
	if err != nil {
		return fmt.Errorf("failed to parse summary template: %w", err)
	}

	if err := tmpl.Execute(&sb, map[string]any{
		"Rows":      finalRows,
		"Truncated": truncated,
	}); err != nil {
		return fmt.Errorf("failed to execute summary template: %w", err)
	}

	// Delete old summary comments
	listOpts := &platform.ListReportsOptions{
		GitHub: &github.IssueListCommentsOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		},
	}

	var allReports []*platform.Report
	for {
		res, err := c.platformClient.ListReports(ctx, prNumber, listOpts)
		if err != nil {
			return fmt.Errorf("failed to list comments on PR [#%d]: %w", prNumber, err)
		}
		allReports = append(allReports, res.Reports...)
		if res.Pagination == nil || res.Pagination.NextPage == 0 {
			break
		}
		listOpts.GitHub.Page = res.Pagination.NextPage
	}

	for _, r := range allReports {
		if strings.HasPrefix(r.Body, commentPrefix) {
			if err := c.platformClient.DeleteReport(ctx, r.ID); err != nil {
				c.Errf("warning: failed to delete old summary comment: %v", err)
			}
		}
	}

	// Post the new summary report comment
	if err := c.platformClient.CreateReport(ctx, prNumber, sb.String()); err != nil {
		return fmt.Errorf("failed to post summary report on PR [#%d]: %w", prNumber, err)
	}

	c.Outf("successfully posted summary report on PR [#%d]", prNumber)
	return nil
}

var forbiddenArtifactFiles = map[string]struct{}{
	"tfplan.binary": {},
	"tfplan.json":   {},
}

func loadPlanSummaries(artifactsDir string) (map[string]*plansummary.Summary, error) {
	summaries := make(map[string]*plansummary.Summary)

	if _, err := os.Stat(artifactsDir); errors.Is(err, fs.ErrNotExist) {
		return summaries, nil
	}

	if err := filepath.WalkDir(artifactsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if _, ok := forbiddenArtifactFiles[name]; ok {
			return fmt.Errorf("found Terraform plan file %s in artifacts directory: "+
				"plan files contain sensitive values and must not be uploaded as artifacts, "+
				"upload only %s files (see the guardian-plan.yml workflow template)", p, plansummary.Filename)
		}
		if name != plansummary.Filename {
			return nil
		}

		s, err := plansummary.Read(p)
		if err != nil {
			return fmt.Errorf("failed to read plan summary: %w", err)
		}
		if _, ok := summaries[s.Entrypoint]; ok {
			return fmt.Errorf("found multiple plan summaries for entrypoint %s", s.Entrypoint)
		}
		summaries[s.Entrypoint] = s
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to load plan summaries from %s: %w", artifactsDir, err)
	}
	return summaries, nil
}

const planSummaryTemplate = "#### 🔱 Guardian 🔱 **`PLAN SUMMARY`**\n\n" +
	"| Directory | Status | Stats | Notes | Log |\n" +
	"| :--- | :--- | :--- | :--- | :--- |\n" +
	"{{range .Rows}}" +
	"| `{{.Directory}}` | <span style=\"white-space: nowrap;\">{{.Status}}</span> | {{.Stats}} | {{.Notes}} | {{.LogLink}} |\n" +
	"{{end}}" +
	"{{if .Truncated}}\n\n> ⚠️ **Notice**: The report was truncated due to the character limit. Please check the full list and output artifacts in GitHub Actions logs.\n{{end}}"

const applySummaryTemplate = "#### 🔱 Guardian 🔱 **`APPLY SUMMARY`**\n\n" +
	"| Directory | Status | Notes | Log |\n" +
	"| :--- | :--- | :--- | :--- |\n" +
	"{{range .Rows}}" +
	"| `{{.Directory}}` | <span style=\"white-space: nowrap;\">{{.Status}}</span> | {{.Notes}} | {{.LogLink}} |\n" +
	"{{end}}" +
	"{{if .Truncated}}\n\n> ⚠️ **Notice**: The report was truncated due to the character limit.\n{{end}}"

type summaryRow struct {
	Directory string
	Status    string
	Stats     string
	Notes     string
	LogLink   string
}
