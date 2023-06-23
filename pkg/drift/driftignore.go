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

// ignoredProjectPattern is a Regex pattern used to identify projects that should be ignored.
var ignoredProjectPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/projects\/(\d*)$`)

// ignoredFolderPattern is a  Regex pattern used to identify folders that should be ignored.
var ignoredFolderPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/folders\/(\d*)$`)

// filterIgnored removes any asset iam that is in the ignored assets.
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

// expandGraph traverses the asset hierarchy graph and adds any nested folders or projects beneath every ignored asset.
func expandGraph(ignored *ignoredAssets, hierarchyGraph *assets.HierarchyGraph) (*ignoredAssets, error) {
	ignoredProjects := ignored.projects
	ignoredFolders := ignored.folders

	// Traverse the hierarchy
	for folderID := range ignored.folders {
		ids, err := assets.FoldersBeneath(folderID, hierarchyGraph)
		if err != nil {
			return nil, fmt.Errorf("failed to traverse hierarchy for folder with ID %s: %w", folderID, err)
		}
		mergeSets(ignoredFolders, ids)
	}
	for folderID := range ignoredFolders {
		addListToSet(ignoredProjects, hierarchyGraph.IDToNodes[folderID].ProjectIDs)
	}

	return &ignoredAssets{
		iamAssets: ignored.iamAssets,
		projects:  ignoredProjects,
		folders:   ignoredFolders,
	}, nil
}

// driftignore parses the driftignore file into a set.
// Go doesn't implement set so we use a map of boolean values all set to true.
// TODO(dcreey): Consider using yaml/json config https://github.com/abcxyz/guardian/issues/105
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

func addListToSet(set map[string]struct{}, list []string) {
	for _, v := range list {
		set[v] = struct{}{}
	}
}

func mergeSets(setA, setB map[string]struct{}) {
	for i := range setB {
		setA[i] = struct{}{}
	}
}
