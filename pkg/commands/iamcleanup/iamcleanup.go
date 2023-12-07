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
	"github.com/abcxyz/pkg/workerpool"
)

type IAMCleaner struct {
	assetInventoryClient  assetinventory.AssetInventory
	iamClient             iam.IAM
	maxConcurrentRequests int64
}

var allowedRequestFieldsInCoditionExpression = map[string]struct{}{"time": {}}

// Do finds all IAM matching the given scope and query and deletes them.
func (c *IAMCleaner) Do(
	ctx context.Context,
	scope string,
	iamQuery string,
	evaluateCondition bool,
) error {
	logger := logging.FromContext(ctx)
	iams, err := c.assetInventoryClient.IAM(ctx, scope, iamQuery)
	if err != nil {
		return fmt.Errorf("failed to get iam: %w", err)
	}

	logger.DebugContext(ctx, "got IAM",
		"number_of_iam_memberships", len(iams),
		"scope", scope,
		"query", iamQuery)

	// We group by URI to confirm we only delete 1 ResourceID/Member combination at a single time.
	// Multiple concurrent requests to delete IAM for a particular ResourceID/Member result in a conflict.
	var iamsToDelete map[string][]*assetinventory.AssetIAM
	if evaluateCondition {
		iamsToDelete = groupByURI(filterByEvaluation(ctx, iams))
	} else {
		iamsToDelete = groupByURI(iams)
	}

	logger.DebugContext(ctx, "cleaning up IAM",
		"number_of_iam_memberships_to_remove", countAll(iamsToDelete))

	w := workerpool.New[*workerpool.Void](&workerpool.Config{
		Concurrency: c.maxConcurrentRequests,
		StopOnError: true,
	})
	for _, is := range iamsToDelete {
		iamMemberships := is
		if err := w.Do(ctx, func() (*workerpool.Void, error) {
			for _, iamMembership := range iamMemberships {
				switch iamMembership.ResourceType {
				case assetinventory.Organization:
					if err := c.iamClient.RemoveOrganizationIAM(ctx, iamMembership); err != nil {
						return nil, fmt.Errorf("failed to remove org IAM: %w", err)
					}
				case assetinventory.Folder:
					if err := c.iamClient.RemoveFolderIAM(ctx, iamMembership); err != nil {
						return nil, fmt.Errorf("failed to remove folder IAM: %w", err)
					}
				case assetinventory.Project:
					if err := c.iamClient.RemoveProjectIAM(ctx, iamMembership); err != nil {
						return nil, fmt.Errorf("failed to remove project IAM: %w", err)
					}
				default:
					return nil, fmt.Errorf("unable to remove membership for unsupported resource type %s", iamMembership.ResourceType)
				}
			}
			return nil, nil
		}); err != nil {
			return fmt.Errorf("failed to execute remove IAM task: %w", err)
		}
	}
	if _, err := w.Done(ctx); err != nil {
		return fmt.Errorf("failed to execute IAM Removal tasks in parallel: %w", err)
	}

	logger.DebugContext(ctx, "successfully deleted IAM",
		"number_of_iam_memberships_removed", len(iamsToDelete))

	return nil
}

func filterByEvaluation(ctx context.Context, iams []*assetinventory.AssetIAM) []*assetinventory.AssetIAM {
	logger := logging.FromContext(ctx)
	var filteredResults []*assetinventory.AssetIAM
	for _, i := range iams {
		passed, err := evaluateIAMConditionExpression(ctx, i.Condition.Expression)
		if err != nil {
			logger.WarnContext(ctx, "failed to parse expression (CEL) for IAM membership",
				"membership", i,
				"error", err)
		} else if !*passed {
			filteredResults = append(filteredResults, i)
		}
	}
	return filteredResults
}

func evaluateIAMConditionExpression(ctx context.Context, expression string) (*bool, error) {
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

	pe, err := cel.AstToParsedExpr(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Expression (CEL): %w", err)
	}

	for _, arg := range pe.Expr.GetCallExpr().Args {
		if arg.GetSelectExpr() == nil {
			continue
		}
		if _, ok := allowedRequestFieldsInCoditionExpression[arg.GetSelectExpr().Field]; !ok {
			return nil, fmt.Errorf("unsupported field '%s' in Condition Expression. Allowed Request fields: '%s'",
				arg.GetSelectExpr().Field, allowedRequestFieldsInCoditionExpression)
		}
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

	return &passed, nil
}

func groupByURI(iams []*assetinventory.AssetIAM) map[string][]*assetinventory.AssetIAM {
	groupedIAMs := make(map[string][]*assetinventory.AssetIAM)
	for _, i := range iams {
		uri := uriNoRole(i)
		if _, ok := groupedIAMs[uri]; !ok {
			groupedIAMs[uri] = []*assetinventory.AssetIAM{}
		}
		groupedIAMs[uri] = append(groupedIAMs[uri], i)
	}
	return groupedIAMs
}

func uriNoRole(iamMember *assetinventory.AssetIAM) string {
	return fmt.Sprintf("%s/%s/%s", iamMember.ResourceType, iamMember.ResourceID, iamMember.Member)
}

func countAll(iamsByURI map[string][]*assetinventory.AssetIAM) int {
	cnt := 0
	for _, iams := range iamsByURI {
		cnt += len(iams)
	}
	return cnt
}
