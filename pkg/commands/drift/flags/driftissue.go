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

package flags

import (
	"github.com/abcxyz/pkg/cli"
)

// DriftIssueFlags represent the shared drift github issue flags among all commands.
// Embed this struct into any commands that needs to create drift GitHub issues.
type DriftIssueFlags struct {
	FlagSkipGitHubIssue            bool
	FlagGitHubIssueLabels          []string
	FlagGitHubIssueAssignees       []string
	FlagGitHubCommentMessageAppend string
}

func (d *DriftIssueFlags) Register(set *cli.FlagSet) {
	f := set.NewSection("DRIFT ISSUE OPTIONS")

	f.BoolVar(&cli.BoolVar{
		Name:    "skip-github-issue",
		Target:  &d.FlagSkipGitHubIssue,
		Example: "true",
		Usage:   `Whether or not to create a GitHub Issue when a drift is detected.`,
		Default: false,
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-issue-assignees",
		Target:  &d.FlagGitHubIssueAssignees,
		Example: "dcreey",
		Usage:   `The assignees to assign to for any created GitHub Issues.`,
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-issue-labels",
		Target:  &d.FlagGitHubIssueLabels,
		Example: "guardian-iam-drift",
		Usage:   `The labels to use on any created GitHub Issues.`,
		Default: []string{},
	})

	f.StringVar(&cli.StringVar{
		Name:    "github-comment-message-append",
		Target:  &d.FlagGitHubCommentMessageAppend,
		Example: "@dcreey, @my-org/my-team",
		Usage:   `Any arbitrary string message to append to the drift GitHub comment.`,
	})
}
