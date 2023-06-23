// Copyright 2023 Google LLC
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

package drift

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/abcxyz/pkg/logging"

	"github.com/abcxyz/guardian/pkg/drift/assets"
	"github.com/abcxyz/guardian/pkg/drift/iam"
)

type ignoredAssets struct {
	iamAssets map[string]struct{}
	projects  map[string]struct{}
	folders   map[string]struct{}
}

type hierarchyNode struct {
	*assets.HierarchyNode
	projectIDs []string
	folderIDs  []string
}

// ignoredProjectPattern is a Regex pattern used to identify projects that should be ignored.
var ignoredProjectPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/projects\/(\d*)$`)

// ignoredFolderPattern is a  Regex pattern used to identify folders that should be ignored.
var ignoredFolderPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/folders\/(\d*)$`)

func filterIgnored(values map[string]*iam.AssetIAM, ignored *ignoredAssets) map[string]*iam.AssetIAM {
	filtered := make(map[string]*iam.AssetIAM)
	for k, a := range values {
		if a.ResourceType == assets.Project {
			if _, ok := ignored.projects[a.ResourceID]; !ok {
				filtered[k] = a
			}
		} else if a.ResourceType == assets.Folder {
			if _, ok := ignored.folders[a.ResourceID]; !ok {
				filtered[k] = a
			}
		} else { // Handle default case so we do not accidentally drop values.
			filtered[k] = a
		}
	}

	return filtered
}

func expandGraph(ignored *ignoredAssets, hierarchyGraph map[string]*hierarchyNode) (*ignoredAssets, error) {
	ignoredProjects := ignored.projects
	ignoredFolders := ignored.folders

	// Traverse the hierarchy
	for folderID := range ignored.folders {
		ids, err := foldersBeneath(folderID, hierarchyGraph)
		if err != nil {
			return nil, fmt.Errorf("failed to traverse hierarchy for folder with ID %s: %w", folderID, err)
		}
		for i := range ids {
			ignoredFolders[i] = struct{}{}
		}
	}
	for folderID := range ignoredFolders {
		for _, projectID := range hierarchyGraph[folderID].projectIDs {
			ignoredProjects[projectID] = struct{}{}
		}
	}

	return &ignoredAssets{
		iamAssets: ignored.iamAssets,
		projects:  ignoredProjects,
		folders:   ignoredFolders,
	}, nil
}

func foldersBeneath(folderID string, hierarchyGraph map[string]*hierarchyNode) (map[string]struct{}, error) {
	foundIDs := make(map[string]struct{})
	if _, ok := hierarchyGraph[folderID]; !ok {
		return nil, fmt.Errorf("missing reference for folder with ID %s", folderID)
	}
	folderIDs := hierarchyGraph[folderID].folderIDs
	for _, id := range folderIDs {
		ids, err := foldersBeneath(id, hierarchyGraph)
		if err != nil {
			return nil, fmt.Errorf("failed to find folders Beneath folder with ID %s: %w", id, err)
		}
		foundIDs[id] = struct{}{}
		for i := range ids {
			foundIDs[i] = struct{}{}
		}
	}
	return foundIDs, nil
}

// driftignore parses the driftignore file into a set.
// Go doesn't implement set so we use a map of boolean values all set to true.
func driftignore(ctx context.Context, fname string) (*ignoredAssets, error) {
	iamAssets := make(map[string]struct{})
	projects := make(map[string]struct{})
	folders := make(map[string]struct{})
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			logger := logging.FromContext(ctx)
			logger.Debugw("failed to find driftignore", "filename", fname)
			return &ignoredAssets{
				iamAssets,
				projects,
				folders,
			}, nil
		}
		return nil, fmt.Errorf("failed to read driftignore file %s: %w", fname, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		iamAssets[line] = struct{}{}

		projectMatches := ignoredProjectPattern.FindStringSubmatch(line)
		if len(projectMatches) == 2 {
			projects[projectMatches[1]] = struct{}{}
		}

		folderMatches := ignoredFolderPattern.FindStringSubmatch(line)
		if len(folderMatches) == 2 {
			folders[folderMatches[1]] = struct{}{}
		}
	}

	return &ignoredAssets{
		iamAssets,
		projects,
		folders,
	}, nil
}

func newHierarchyGraph(organizationID string, folders, projects []*assets.HierarchyNode) (map[string]*hierarchyNode, error) {
	graph := make(map[string]*hierarchyNode)

	folderHash := make(map[string]*assets.HierarchyNode)
	for _, folder := range folders {
		folderHash[folder.ID] = folder
	}

	graph[organizationID] = &hierarchyNode{
		HierarchyNode: &assets.HierarchyNode{
			ID:         organizationID,
			Name:       "Organization",
			ParentID:   "",
			ParentType: "",
		},
		projectIDs: []string{},
		folderIDs:  []string{},
	}

	for _, folder := range folders {
		addFolderToGraph(graph, folder, folderHash)
	}

	for _, project := range projects {
		if _, ok := graph[project.ParentID]; !ok {
			return nil, fmt.Errorf("missing reference for %s node ID %s", project.ParentType, project.ParentID)
		}
		graph[project.ParentID].projectIDs = append(graph[project.ParentID].projectIDs, project.ID)
	}

	return graph, nil
}

func addFolderToGraph(graph map[string]*hierarchyNode, folder *assets.HierarchyNode, folders map[string]*assets.HierarchyNode) error {
	// Already added.
	if _, ok := graph[folder.ID]; ok {
		return nil
	}

	// Need to add parent node.
	if _, ok := graph[folder.ParentID]; !ok {
		if _, ok := folders[folder.ParentID]; !ok {
			return fmt.Errorf("missing reference for folder ID %s and Name %s", folder.ParentID, folder.Name)
		}
		if err := addFolderToGraph(graph, folders[folder.ParentID], folders); err != nil {
			return fmt.Errorf("failed to add folder %s to graph: %w", folder.ParentID, err)
		}
	}

	graph[folder.ID] = &hierarchyNode{
		HierarchyNode: folder,
		projectIDs:    []string{},
		folderIDs:     []string{},
	}

	graph[folder.ParentID].folderIDs = append(graph[folder.ParentID].folderIDs, folder.ID)

	return nil
}
