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

	"github.com/abcxyz/guardian/pkg/drift"
)

var _ cli.Command = (*DetectIamDriftCommand)(nil)

type DetectIamDriftCommand struct {
	cli.BaseCommand

	// testFlagSetOpts is only used for testing.
	testFlagSetOpts []cli.Option

	organizationID string
	gcsBucketQuery string
}

func (c *DetectIamDriftCommand) Desc() string {
	return `Detect IAM drift in a GCP Organization.`
}

func (c *DetectIamDriftCommand) Help() string {
	return `
Usage: {{ COMMAND }} [options]

  Detect IAM drift in a GCP Organization
`
}

func (c *DetectIamDriftCommand) Flags() *cli.FlagSet {
	set := cli.NewFlagSet(c.testFlagSetOpts...)

	// Command options
	f := set.NewSection("COMMAND OPTIONS")

	f.StringVar(&cli.StringVar{
		Name:    "organization_id",
		Target:  &c.organizationID,
		Example: "123435456456",
		Usage:   `GCP Organization to detect drift for.`,
		Default: "",
	})
	f.StringVar(&cli.StringVar{
		Name:    "gcs_bucket_query",
		Target:  &c.gcsBucketQuery,
		Example: "labels.terraform:*",
		Usage:   `The label to use to find GCS buckets with terraform statefiles.`,
	})

	return set
}

func (c *DetectIamDriftCommand) Run(ctx context.Context, args []string) error {
	f := c.Flags()

	if err := f.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args = f.Args()
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %q", args)
	}

	c.Outf("Running IAM Drift Detection...")

	if c.organizationID == "" {
		return fmt.Errorf("invalid Argument: organization_id must be provided")
	}

	if err := drift.Process(ctx, c.organizationID, c.gcsBucketQuery); err != nil {
		return fmt.Errorf("error detecting drift %w", err)
	}

	return nil
}
