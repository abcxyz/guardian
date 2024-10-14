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
	"github.com/abcxyz/abc-updater/pkg/metrics"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abcxyz/guardian/internal/metricswrap"
	"github.com/abcxyz/guardian/internal/version"
	"github.com/abcxyz/guardian/pkg/commands/apply"
	"github.com/abcxyz/guardian/pkg/commands/cleanup"
	"github.com/abcxyz/guardian/pkg/commands/drift"
	"github.com/abcxyz/guardian/pkg/commands/drift/statefiles"
	"github.com/abcxyz/guardian/pkg/commands/entrypoints"
	"github.com/abcxyz/guardian/pkg/commands/iamcleanup"
	"github.com/abcxyz/guardian/pkg/commands/plan"
	"github.com/abcxyz/guardian/pkg/commands/policy"
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
			"policy": func() cli.Command {
				return &cli.RootCommand{
					Name:        "policy",
					Description: "Perform operations related to policy enforcement",
					Commands: map[string]cli.CommandFactory{
						"enforce": func() cli.Command {
							return &policy.EnforceCommand{}
						},
						"fetch-data": func() cli.Command {
							return &policy.FetchDataCommand{}
						},
					},
				}
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

func setupMetricsClient(ctx context.Context) context.Context {
	mClient, err := metrics.New(ctx, version.Name, version.Version)
	if err != nil {
		logging.FromContext(ctx).DebugContext(ctx, "metric client creation failed", "error", err)
	}

	ctx = metrics.WithClient(ctx, mClient)
	return ctx
}

func realMain(ctx context.Context) error {
	start := time.Now()
	setLogEnvVars()
	ctx = logging.WithLogger(ctx, logging.NewFromEnv("GUARDIAN_"))

	ctx = setupMetricsClient(ctx)
	defer func() {
		if r := recover(); r != nil {
			metricswrap.WriteMetric(ctx, "panics", 1)
			panic(r)
		}
	}()

	metricswrap.WriteMetric(ctx, "runs", 1)
	defer func() {
		// Needs to be wrapped in func() due to time.Since(start).
		metricswrap.WriteMetric(ctx, "runtime_millis", time.Since(start).Milliseconds())
	}()

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
