// Copyright 2022 Google LLC
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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/abcxyz/guardian/pkg/drift"
)

const lintCommandHelp = `
The "lint" command 
EXAMPLES
  detect-iam-drift <organization_id> <gcs_bucket_query>
FLAGS
`

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func realMain() error {
	ctx, done := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer done()

	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(lintCommandHelp))
		f.PrintDefaults()
	}
	organizationID := f.Int64("organization_id", 0, "GCP Organization to detect drift for")
	bucketQuery := f.String("gcs_bucket_query", "labels.terraform:*", "the label to use to find GCS buckets with terraform statefiles")

	if err := f.Parse(os.Args[1:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}
	if *organizationID == 0 {
		return fmt.Errorf("Invalid Argument: organization_id must be provided")
	}

	if err := drift.Process(ctx, *organizationID, *bucketQuery); err != nil {
		return fmt.Errorf("error detecting drift %w", err)
	}
	return nil
}
