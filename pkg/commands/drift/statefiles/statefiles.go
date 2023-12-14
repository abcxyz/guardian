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

// Package statefiles provide the drift detection on Terraform statefile functionality for Guardian.
package statefiles

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"

	githubAPI "github.com/google/go-github/v53/github"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/commands/drift"
	driftflags "github.com/abcxyz/guardian/pkg/commands/drift/flags"
	"github.com/abcxyz/guardian/pkg/flags"
	"github.com/abcxyz/guardian/pkg/git"
	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/guardian/pkg/storage"
	"github.com/abcxyz/guardian/pkg/terraform"
	"github.com/abcxyz/guardian/pkg/terraform/parser"
	"github.com/abcxyz/guardian/pkg/util"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/sets"
	"github.com/abcxyz/pkg/workerpool"
)

var _ cli.Command = (*DriftStatefilesCommand)(nil)

const (
	issueTitle = "Terraform statefile drift detected"
	issueBody  = `We've detected a drift between the statefiles stored in your GCS bucket and the
        ones referenced in your backend config blocks.

        See the comment(s) below to see details of the drift

        Please determine which parts are correct, and delete or rename any unused statefiles
        in your GCS bucket.

        Re-run drift detection manually once complete to verify all diffs are properly resolved.`
	// maxConcurrentRequests is the maximum number of concurrent gcs reads to perform at once.
	maxConcurrentRequests = 10
)

type DriftStatefilesCommand struct {
	cli.BaseCommand

	directory    string
	tmpDirectory string

	flags.GitHubFlags
	flags.RetryFlags
	driftflags.DriftIssueFlags

	flagOrganizationID                string
	flagGCSBucketQuery                string
	flagDetectGCSBucketsFromTerraform bool
	flagTerraformRepoTopics           []string
	flagIgnoreDirPatterns             []string

	parsedFlagIgnoreDirPatters []*regexp.Regexp

	assetInventoryClient assetinventory.AssetInventory
	gitClient            git.Git
	githubClient         github.GitHub
	issueService         *drift.GitHubDriftIssueService
	terraformParser      parser.Terraform
}

func (c *DriftStatefilesCommand) Desc() string {
	return `Run the drift detection algorithm on all terraform statefiles in a directory`
}

func (c *DriftStatefilesCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options] <directory>

  Run the drift detection for terraform statefiles in a directory.
`
}

func (c *DriftStatefilesCommand) Flags() *cli.FlagSet {
	set := c.NewFlagSet()

	c.GitHubFlags.Register(set)
	c.RetryFlags.Register(set)
	c.DriftIssueFlags.Register(set)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "organization-id",
		Target:  &c.flagOrganizationID,
		Example: "123435456456",
		Usage:   `The Google Cloud organization ID for which to detect drift.`,
	})

	f.StringVar(&cli.StringVar{
		Name:    "gcs-bucket-query",
		Target:  &c.flagGCSBucketQuery,
		Example: "labels.terraform:*",
		Usage:   `The label to use to find GCS buckets with Terraform statefiles.`,
	})

	f.BoolVar(&cli.BoolVar{
		Name:   "detect-gcs-buckets-from-terraform",
		Target: &c.flagDetectGCSBucketsFromTerraform,
		Usage:  `Whether or not to use the terraform backend configs to determine gcs buckets.`,
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "github-repo-terraform-topics",
		Target:  &c.flagTerraformRepoTopics,
		Example: "terraform,guardian",
		Usage:   `Topics to use to identify github repositories that contain terraform configurations.`,
	})

	f.StringSliceVar(&cli.StringSliceVar{
		Name:    "ignore-dir-patterns",
		Target:  &c.flagIgnoreDirPatterns,
		Example: "templates\\/**',test\\/**",
		Usage:   `Directories to filter from the possible terraform entrypoint locations. Paths will be matched against the root of each cloned repository.`,
	})

	set.AfterParse(func(existingErr error) (merr error) {
		for _, p := range c.flagIgnoreDirPatterns {
			r, err := regexp.Compile(p)
			if err != nil {
				merr = errors.Join(merr, fmt.Errorf("failed to compile ignore-dir-patterns: %w", err))
			} else {
				c.parsedFlagIgnoreDirPatters = append(c.parsedFlagIgnoreDirPatters, r)
			}
		}
		if len(c.DriftIssueFlags.FlagGitHubIssueLabels) == 0 {
			c.DriftIssueFlags.FlagGitHubIssueLabels = []string{"guardian-statefile-drift"}
		}
		return merr
	})

	return set
}

func (c *DriftStatefilesCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	parsedArgs := f.Args()

	if len(parsedArgs) != 1 {
		return flag.ErrHelp
	}

	dirAbs, err := util.PathEvalAbs(parsedArgs[0])
	if err != nil {
		return fmt.Errorf("failed to absolute path for directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp(dirAbs, "tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	c.directory = dirAbs
	c.tmpDirectory = tmpDir
	c.gitClient = git.NewGitClient(c.tmpDirectory)
	c.githubClient = github.NewClient(ctx, c.GitHubFlags.FlagGitHubToken)
	c.issueService = drift.NewGitHubDriftIssueService(
		c.githubClient,
		c.GitHubFlags.FlagGitHubOwner,
		c.GitHubFlags.FlagGitHubRepo,
		issueTitle,
		issueBody,
	)
	c.terraformParser, err = parser.NewTerraformParser(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create terraform parser: %w", err)
	}
	c.assetInventoryClient, err = assetinventory.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize assets client: %w", err)
	}

	return c.Process(ctx)
}

// Process handles the main logic for the Guardian init process.
func (c *DriftStatefilesCommand) Process(ctx context.Context) error {
	logger := logging.FromContext(ctx).
		With("github_owner", c.GitHubFlags.FlagGitHubOwner).
		With("github_repo", c.GitHubFlags.FlagGitHubOwner)

	logger.DebugContext(ctx, "starting Guardian drift statefiles")
	if err := c.cloneAllGitHubRepositories(ctx, logger); err != nil {
		return fmt.Errorf("failed to clone github repositories: %w", err)
	}

	logger.DebugContext(ctx, "finding expected statefile uris")
	expectedURIs, err := c.expectedStatefileUris(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to determine expected state file URIs: %w", err)
	}

	logger.DebugContext(ctx, "finding actual statefile uris")
	gotURIs, err := c.actualStatefileUris(ctx, logger, expectedURIs)
	if err != nil {
		return fmt.Errorf("failed to determine actual state file URIs: %w", err)
	}

	// Compare expected vs actual statefiles.
	statefilesNotInRemote := sets.Subtract(expectedURIs, gotURIs)
	statefilesNotInLocal := sets.Subtract(gotURIs, expectedURIs)

	emptyStateFiles, err := c.emptyStateFiles(ctx, statefilesNotInLocal)
	if err != nil {
		return fmt.Errorf("failed to find empty statefiles: %w", err)
	}

	statefilesNotInLocalNotEmpty := sets.Subtract(statefilesNotInLocal, emptyStateFiles)

	sort.Strings(statefilesNotInRemote)
	sort.Strings(statefilesNotInLocalNotEmpty)
	sort.Strings(emptyStateFiles)

	changesDetected := len(statefilesNotInRemote) > 0 || len(statefilesNotInLocalNotEmpty) > 0 || len(emptyStateFiles) > 0
	m := driftMessage(statefilesNotInRemote, statefilesNotInLocalNotEmpty, emptyStateFiles)
	if changesDetected {
		c.Outf(m)
	}

	if c.DriftIssueFlags.FlagSkipGitHubIssue {
		return nil
	}
	if c.DriftIssueFlags.FlagGitHubCommentMessageAppend != "" {
		m = strings.Join([]string{m, c.DriftIssueFlags.FlagGitHubCommentMessageAppend}, "\n\n")
	}
	if changesDetected {
		if err := c.issueService.CreateOrUpdateIssue(ctx, c.DriftIssueFlags.FlagGitHubIssueAssignees, c.DriftIssueFlags.FlagGitHubIssueLabels, m); err != nil {
			return fmt.Errorf("failed to create or update GitHub Issue: %w", err)
		}
	} else {
		if err := c.issueService.CloseIssues(ctx, c.DriftIssueFlags.FlagGitHubIssueLabels); err != nil {
			return fmt.Errorf("failed to close GitHub Issues: %w", err)
		}
	}

	return nil
}

func (c *DriftStatefilesCommand) cloneAllGitHubRepositories(ctx context.Context, logger *slog.Logger) error {
	// Clone all git repositories.
	repositories, err := c.githubClient.ListRepositories(ctx, c.GitHubFlags.FlagGitHubOwner, &githubAPI.RepositoryListByOrgOptions{})
	if err != nil {
		return fmt.Errorf("failed to determine github repositories: %w", err)
	}

	repositoriesWithTerraform := []*github.Repository{}
	for _, r := range repositories {
		if len(sets.Subtract(r.Topics, c.flagTerraformRepoTopics)) == 0 && len(r.Topics) != 0 {
			repositoriesWithTerraform = append(repositoriesWithTerraform, r)
		}
	}
	logger.DebugContext(ctx, "found github repositories matching topics",
		"number_of_candidate_repositories", len(repositories),
		"number_of_matched_repositories", len(repositoriesWithTerraform),
		"topics", c.flagTerraformRepoTopics)

	for _, r := range repositoriesWithTerraform {
		if err = c.gitClient.CloneRepository(ctx, c.GitHubFlags.FlagGitHubToken, r.Owner, r.Name); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}
	return nil
}

func (c *DriftStatefilesCommand) expectedStatefileUris(ctx context.Context, logger *slog.Logger) ([]string, error) {
	// Determine expected statefiles from checked out repositories.
	entrypoints, err := terraform.GetEntrypointDirectories(c.directory, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform directories: %w", err)
	}
	entrypointBackendFiles := make([]string, 0, len(entrypoints))
	for _, e := range entrypoints {
		included := true
		relPath := "." + strings.TrimPrefix(e.BackendFile, c.directory)
		for _, p := range c.parsedFlagIgnoreDirPatters {
			matches := p.FindStringSubmatch(relPath)
			if len(matches) > 0 {
				included = false
			}
		}
		if included {
			entrypointBackendFiles = append(entrypointBackendFiles, e.BackendFile)
		}
	}
	logger.DebugContext(ctx, "terraform entrypoint directories", "entrypoint_backend_files", entrypointBackendFiles)

	expectedURIs := make([]string, 0, len(entrypointBackendFiles))
	var errs []error
	for _, f := range entrypointBackendFiles {
		config, _, err := terraform.ExtractBackendConfig(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse Terraform backend config: %w", err))
			continue
		}
		if config.GCSBucket == nil || *config.GCSBucket == "" {
			errs = append(errs, fmt.Errorf("unsupported backend type for terraform config at %s - only gcs backends are supported", f))
			continue
		}
		expectedURIs = append(expectedURIs, fmt.Sprintf("gs://%s/%s/default.tfstate", *config.GCSBucket, *config.Prefix))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to determine statefile gcs URIs: %w", errors.Join(errs...))
	}

	return expectedURIs, nil
}

func (c *DriftStatefilesCommand) actualStatefileUris(ctx context.Context, logger *slog.Logger, terraformUris []string) ([]string, error) {
	var buckets []string
	var err error
	if c.flagDetectGCSBucketsFromTerraform {
		for _, uri := range terraformUris {
			bucket, _, err := storage.SplitObjectURI(uri)
			if err != nil {
				return nil, fmt.Errorf("failed to parse GCS URI: %w", err)
			}
			buckets = append(buckets, *bucket)
		}
	} else {
		buckets, err = c.assetInventoryClient.Buckets(ctx, c.flagOrganizationID, c.flagGCSBucketQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to determine gcs buckets: %w", err)
		}
	}
	logger.DebugContext(ctx, "finding statefiles in gcs buckets",
		"gcs_buckets", buckets)

	gotURIs, err := c.terraformParser.StateFileURIs(ctx, buckets)
	if err != nil {
		return nil, fmt.Errorf("failed to determine state file URIs for gcs buckets %s: %w", buckets, err)
	}
	return gotURIs, nil
}

func (c *DriftStatefilesCommand) emptyStateFiles(ctx context.Context, gcsURIs []string) ([]string, error) {
	type Resource struct {
		Empty bool
		URI   string
	}
	w := workerpool.New[*Resource](&workerpool.Config{
		Concurrency: maxConcurrentRequests,
		StopOnError: true,
	})
	for _, u := range gcsURIs {
		uri := u
		if err := w.Do(ctx, func() (*Resource, error) {
			empty, err := c.terraformParser.StateWithoutResources(ctx, uri)
			if err != nil {
				return nil, fmt.Errorf("failed to get determine if state file URI has resources: %w", err)
			}
			return &Resource{empty, uri}, nil
		}); err != nil && !errors.Is(err, workerpool.ErrStopped) {
			return nil, fmt.Errorf("failed to execute terraform resources task: %w", err)
		}
	}

	results, err := w.Done(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute terraform resources tasks in parallel: %w", err)
	}

	var emptyStateFiles []string
	var merr error
	for _, r := range results {
		if err := r.Error; err != nil {
			if !errors.Is(err, workerpool.ErrStopped) {
				merr = errors.Join(merr, fmt.Errorf("failed to execute resource task: %w", err))
			}
			continue
		}
		if r.Value.Empty {
			emptyStateFiles = append(emptyStateFiles, r.Value.URI)
		}
	}
	if merr != nil {
		return nil, fmt.Errorf("failed to execute terraform resource tasks in parallel: %w", merr)
	}
	return emptyStateFiles, nil
}

func Set(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

func driftMessage(statefilesNotInRemote, statefilesNotInLocal, emptyStatefilesNotInLocal []string) string {
	var msg strings.Builder
	if len(statefilesNotInRemote) > 0 {
		msg.WriteString(fmt.Sprintf("Found state locally that are not in remote \n> %s", strings.Join(statefilesNotInRemote, "\n> ")))
		if len(statefilesNotInLocal) > 0 || len(emptyStatefilesNotInLocal) > 0 {
			msg.WriteString("\n\n")
		}
	}
	if len(statefilesNotInLocal) > 0 {
		msg.WriteString(fmt.Sprintf("Found statefiles in remote that are not in local \n> %s", strings.Join(statefilesNotInLocal, "\n> ")))
		if len(emptyStatefilesNotInLocal) > 0 {
			msg.WriteString("\n\n")
		}
	}
	if len(emptyStatefilesNotInLocal) > 0 {
		msg.WriteString(fmt.Sprintf("Found empty statefiles in remote that are not in local \n> %s", strings.Join(emptyStatefilesNotInLocal, "\n> ")))
	}
	return msg.String()
}
