// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"fmt"

	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/guardian/pkg/drift"
)

var _ cli.Command = (*DetectIamDriftCommand)(nil)

type DetectIamDriftCommand struct {
	cli.BaseCommand

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option

	flagOrganizationID string
	flagGCSBucketQuery string
}

func (c *DetectIamDriftCommand) Desc() string {
	return `Detect IAM drift in a GCP organization`
}

func (c *DetectIamDriftCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Detect IAM drift in a GCP organization.
`
}

func (c *DetectIamDriftCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "organization-id",
		Target:  &c.flagOrganizationID,
		Example: "123435456456",
		Usage:   `The Google Cloud organization ID for which to detect drift.`,
		Default: "",
	})
	f.StringVar(&cli.StringVar{
		Name:    "gcs-bucket-query",
		Target:  &c.flagGCSBucketQuery,
		Example: "labels.terraform:*",
		Usage:   `The label to use to find GCS buckets with Terraform statefiles.`,
	})

	return set
}

func (c *DetectIamDriftCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()

	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	logger := logging.FromContext(ctx)

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	logger.Debugw("Running IAM Drift Detection...",
		"name", version.Name,
		"commit", version.Commit,
		"version", version.Version)

	if c.flagOrganizationID == "" {
		return fmt.Errorf("missing -organization-id")
	}

	if err := drift.Process(ctx, c.flagOrganizationID, c.flagGCSBucketQuery); err != nil {
		return fmt.Errorf("failed to detect drift %w", err)
	}

	return nil
}
