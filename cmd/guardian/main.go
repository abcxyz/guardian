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
	"github.com/abcxyz/guardian/pkg/commands/apply"
	"github.com/abcxyz/guardian/pkg/commands/cleanup"
	"github.com/abcxyz/guardian/pkg/commands/drift"
	"github.com/abcxyz/guardian/pkg/commands/drift/statefiles"
	"github.com/abcxyz/guardian/pkg/commands/entrypoints"
	"github.com/abcxyz/guardian/pkg/commands/iamcleanup"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/commands/run"
	"github.com/abcxyz/guardian/pkg/commands/workflows"
	"github.com/abcxyz/pkg/cli"
	"github.com/abcxyz/pkg/logging"
)

const (
	defaultLogLevel  = "warn"
	defaultLogFormat = "json"
	defaultLogDebug  = "false"
)

// rootCmd defines the starting command structure.
var rootCmd = func() cli.Command {
	return &cli.RootCommand{
		Name:    "guardian",
		Version: version.HumanVersion,
		Commands: map[string]cli.CommandFactory{
			"entrypoints": func() cli.Command {
				return &entrypoints.EntrypointsCommand{}
			},
			"workflows": func() cli.Command {
				return &cli.RootCommand{
					Name:        "workflows",
					Description: "Perform operations related to running Guardian using workflows",
					Commands: map[string]cli.CommandFactory{
						"remove-guardian-comments": func() cli.Command {
							return &workflows.RemoveGuardianCommentsCommand{}
						},
						"validate-permissions": func() cli.Command {
							return &workflows.ValidatePermissionsCommand{}
						},
						"plan-status-comment": func() cli.Command {
							return &workflows.PlanStatusCommentCommand{}
						},
					},
				}
			},
			"plan": func() cli.Command {
				return &plan.PlanCommand{}
			},
			"apply": func() cli.Command {
				return &apply.ApplyCommand{}
			},
			"iam": func() cli.Command {
				return &cli.RootCommand{
					Name:        "iam",
					Description: "Perform operations related to iam",
					Commands: map[string]cli.CommandFactory{
						"detect-drift": func() cli.Command {
							return &drift.DetectIamDriftCommand{}
						},
						"cleanup": func() cli.Command {
							return &iamcleanup.IAMCleanupCommand{}
						},
					},
				}
			},
			"drift": func() cli.Command {
				return &cli.RootCommand{
					Name:        "drift",
					Description: "Perform operations related to drift",
					Commands: map[string]cli.CommandFactory{
						"statefiles": func() cli.Command {
							return &statefiles.DriftStatefilesCommand{}
						},
					},
				}
			},
			"cleanup": func() cli.Command {
				return &cleanup.CleanupCommand{}
			},
			"run": func() cli.Command {
				return &run.RunCommand{}
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
	if os.Getenv("GUARDIAN_LOG_FORMAT") == "" {
		os.Setenv("GUARDIAN_LOG_FORMAT", defaultLogFormat)
	}

	if os.Getenv("GUARDIAN_LOG_LEVEL") == "" {
		os.Setenv("GUARDIAN_LOG_LEVEL", defaultLogLevel)
	}

	if os.Getenv("GUARDIAN_LOG_DEBUG") == "" {
		os.Setenv("GUARDIAN_LOG_DEBUG", defaultLogDebug)
	}
}
