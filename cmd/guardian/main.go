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

// Package main is the main entrypoint to the application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/guardian/pkg/commands/drift"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

const (
	defaultLogLevel = "warn"
	defaultLogMode  = "development"
)

// rootCmd defines the starting command structure.
var rootCmd = func() cli.Command {
	return &cli.RootCommand{
		Name:    "guardian",
		Version: version.HumanVersion,
		Commands: map[string]cli.CommandFactory{
			"plan": func() cli.Command {
				return &cli.RootCommand{
					Name:        "plan",
					Description: "Perform operations related to Terraform planning",
					Commands: map[string]cli.CommandFactory{
						"init": func() cli.Command {
							return &plan.PlanInitCommand{}
						},
						"run": func() cli.Command {
							return &plan.PlanRunCommand{}
						},
					},
				}
			},
			// "apply": func() cli.Command {
			// 	return &ApplyCommand{}
			// },
			"detect-iam-drift": func() cli.Command {
				return &drift.DetectIamDriftCommand{}
			},
		},
	}
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer done()

	if err := realMain(ctx); err != nil {
		done()
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func realMain(ctx context.Context) error {
	setLogEnvVars()
	ctx = logging.WithLogger(ctx, logging.NewFromEnv("GUARDIAN_"))
	return rootCmd().Run(ctx, os.Args[1:]) //nolint:wrapcheck // Want passthrough
}

// setLogEnvVars set the logging environment variables to their default
// values if not provided.
func setLogEnvVars() {
	if os.Getenv("GUARDIAN_LOG_MODE") == "" {
		os.Setenv("GUARDIAN_LOG_MODE", defaultLogMode)
	}

	if os.Getenv("GUARDIAN_LOG_LEVEL") == "" {
		os.Setenv("GUARDIAN_LOG_LEVEL", defaultLogLevel)
	}
}
