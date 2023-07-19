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

package iamcleanup

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/guardian/pkg/iam"
	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/worker"
)

type IAMCleaner struct {
	assetInventoryClient  assetinventory.AssetInventory
	iamClient             iam.IAM
	maxConcurrentRequests int64
}

func NewIAMCleaner(ctx context.Context, maxConcurrentRequests int64, workingDirectory string) (*IAMCleaner, error) {
	assetInventoryClient, err := assetinventory.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize assets client: %w", err)
	}

	iamClient, err := iam.NewClient(ctx, &workingDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iam client: %w", err)
	}

	return &IAMCleaner{
		assetInventoryClient,
		iamClient,
		maxConcurrentRequests,
	}, nil
}

// Do finds all IAM matching the given scope and query and deletes them.
func (c *IAMCleaner) Do(
	ctx context.Context,
	scope string,
	iamQuery string,
	expiredOnly bool,
) error {
	logger := logging.FromContext(ctx)
	iams, err := c.assetInventoryClient.IAM(ctx, scope, iamQuery)
	if err != nil {
		return fmt.Errorf("failed to get iam: %w", err)
	}

	logger.Debugw("got IAM",
		"number_of_iam_memberships", len(iams),
		"scope", scope,
		"query", iamQuery)

	w := worker.New[*worker.Void](c.maxConcurrentRequests)
	var expiredIAMs []*assetinventory.AssetIAM

	for _, i := range iams {
		expired, err := isIAMConditionExpressionExpired(ctx, i.Condition.Expression)
		if err != nil {
			logger.Warnw("Failed to parse expression (CEL) for IAM membership", "membership", i, "error", err)
		} else if *expired {
			expiredIAMs = append(expiredIAMs, i)
		}
	}

	logger.Debugw("Cleaning up expired IAM", "number_of_iam_memberships", len(expiredIAMs))

	for _, i := range expiredIAMs {
		iamMembership := i
		if err := w.Do(ctx, func() (*worker.Void, error) {
			if iamMembership.ResourceType == assetinventory.Organization {
				if err := c.iamClient.RemoveOrganizationIAM(ctx, iamMembership); err != nil {
					return nil, fmt.Errorf("failed to remove org IAM: %w", err)
				}
			} else if iamMembership.ResourceType == assetinventory.Folder {
				if err := c.iamClient.RemoveFolderIAM(ctx, iamMembership); err != nil {
					return nil, fmt.Errorf("failed to remove folder IAM: %w", err)
				}
			} else if iamMembership.ResourceType == assetinventory.Project {
				if err := c.iamClient.RemoveProjectIAM(ctx, iamMembership); err != nil {
					return nil, fmt.Errorf("failed to remove project IAM: %w", err)
				}
			}
			return nil, nil
		}); err != nil {
			return fmt.Errorf("failed to execute remove IAM task: %w", err)
		}
	}
	return nil
}

func isIAMConditionExpressionExpired(ctx context.Context, expression string) (*bool, error) {
	// Example: request.time < timestamp("2019-01-01T00:00:00Z")
	env, err := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Variable("request",
			cel.ObjectType("google.rpc.context.AttributeContext.Request"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Expression (CEL) environment: %w", err)
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile Expression (CEL): %w", issues.Err())
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create Expression (CEL) program: %w", err)
	}
	val, _, err := program.ContextEval(ctx, map[string]any{"request": &rpcpb.AttributeContext_Request{Time: timestamppb.Now()}})
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate Expression (CEL): %w", err)
	}
	passed, ok := val.Value().(bool)
	if !ok {
		return nil, fmt.Errorf("failed to parse evaluation from Expression (CEL) with value: %s", val)
	}
	expired := !passed

	return &expired, nil
}
