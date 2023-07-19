package iamcleanup

import (
	"context"
	"fmt"
	"log"

	"github.com/google/cel-go/cel"
	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"

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

	w := worker.New[*worker.Void](c.maxConcurrentRequests)
	filteredIAMs := iams

	env, err := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Variable("request",
			cel.ObjectType("google.rpc.context.AttributeContext.Request"),
		),
	)
	ast, issues := env.Compile(`name.startsWith("/groups/" + group)`)
	if issues != nil && issues.Err() != nil {
		log.Fatalf("type-check error: %s", issues.Err())
	}

	for _, i := range filteredIAMs {
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

	logger.Debugw("got IAM",
		"number_of_iam_memberships", len(iams),
		"scope", scope,
		"query", iamQuery)
	return nil
}
