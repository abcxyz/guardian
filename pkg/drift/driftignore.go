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
	iamAssets  map[string]struct{}
	projectIDs map[string]struct{}
	folderIDs  map[string]struct{}
	roles      map[string]struct{}
}

// ignoredProjectPattern is a Regex pattern used to identify projects that should be ignored.
var ignoredProjectPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/projects\/([^\/]*)$`)

// ignoredFolderPattern is a  Regex pattern used to identify folders that should be ignored.
var ignoredFolderPattern = regexp.MustCompile(`^\/organizations\/(?:\d*)\/folders\/([^\/]*)$`)

// ignoredFolderPattern is a  Regex pattern used to identify folders that should be ignored.
// Example: /roles/owner/serviceAccount:platform-ops-tfa-sa-ef3e@platform-ops-ef3e.iam.gserviceaccount.com
// Example: /roles/resourcemanager.folderEditor/serviceAccount:platform-ops-tfa-sa-ef3e@platform-ops-ef3e.iam.gserviceaccount.com
var ignoredRolesPattern = regexp.MustCompile(`^\/roles\/([^\/\s]*)\/(serviceAccount|group|user)\:([^\/\s]*)$`)

var defaultURIFilterPatterns = []*regexp.Regexp{
	regexp.MustCompile(`artifactregistry.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-artifactregistry.iam.gserviceaccount.com`),
	regexp.MustCompile(`bigquerydatatransfer.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-bigquerydatatransfer.iam.gserviceaccount.com`),
	regexp.MustCompile(`binaryauthorization.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-binaryauthorization.iam.gserviceaccount.com`),
	regexp.MustCompile(`cloudbuild.builds.builder\/serviceAccount:(?:\d*)@cloudbuild.gserviceaccount.com`),
	regexp.MustCompile(`cloudbuild.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudbuild.iam.gserviceaccount.com`),
	regexp.MustCompile(`cloudfunctions.serviceAgent\/serviceAccount:service-(?:\d*)@gcf-admin-robot.iam.gserviceaccount.com`),
	regexp.MustCompile(`cloudkms.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudkms.iam.gserviceaccount.com`),
	regexp.MustCompile(`cloudscheduler.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudscheduler.iam.gserviceaccount.com`),
	regexp.MustCompile(`compute.networkViewer\/serviceAccount:(?:\d*)-compute@developer.gserviceaccount.com`),
	regexp.MustCompile(`compute.serviceAgent\/serviceAccount:service-(?:\d*)@compute-system.iam.gserviceaccount.com`),
	regexp.MustCompile(`container.serviceAgent\/serviceAccount:service-(?:\d*)@container-engine-robot.iam.gserviceaccount.com`),
	regexp.MustCompile(`containeranalysis.ServiceAgent\/serviceAccount:service-(?:\d*)@container-analysis.iam.gserviceaccount.com`),
	regexp.MustCompile(`containerregistry.ServiceAgent\/serviceAccount:service-(?:\d*)@containerregistry.iam.gserviceaccount.com`),
	regexp.MustCompile(`containerthreatdetection.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-ktd-control.iam.gserviceaccount.com`),
	regexp.MustCompile(`dataflow.serviceAgent\/serviceAccount:service-(?:\d*)@dataflow-service-producer-prod.iam.gserviceaccount.com`),
	regexp.MustCompile(`editor\/serviceAccount:(?:\d*)-compute@developer.gserviceaccount.com`),
	regexp.MustCompile(`editor\/serviceAccount:(?:\d*)@cloudservices.gserviceaccount.com`),
	regexp.MustCompile(`file.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-filer.iam.gserviceaccount.com`),
	regexp.MustCompile(`firebase.managementServiceAgent\/serviceAccount:firebase-service-account@firebase-sa-management.iam.gserviceaccount.com`),
	regexp.MustCompile(`firebase.managementServiceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-firebase.iam.gserviceaccount.com`),
	regexp.MustCompile(`firebaserules.system\/serviceAccount:service-(?:\d*)@firebase-rules.iam.gserviceaccount.com`),
	regexp.MustCompile(`.*iap.settingsAdmin\/serviceAccount:(?:\d*)-compute@developer.gserviceaccount.com`),
	regexp.MustCompile(`identitytoolkit.viewer\/serviceAccount:(?:\d*)-compute@developer.gserviceaccount.com`),
	regexp.MustCompile(`networkmanagement.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-networkmanagement.iam.gserviceaccount.com`),
	regexp.MustCompile(`pubsub.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-pubsub.iam.gserviceaccount.com`),
	regexp.MustCompile(`redis.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-redis.iam.gserviceaccount.com`),
	regexp.MustCompile(`run.serviceAgent\/serviceAccount:service-(?:\d*)@serverless-robot-prod.iam.gserviceaccount.com`),
	regexp.MustCompile(`securitycenter.serviceAgent\/serviceAccount:service-org-(?:\d*)@security-center-api.iam.gserviceaccount.com`),
	regexp.MustCompile(`securitycenter.serviceAgent\/serviceAccount:service-project-(?:\d*)@security-center-api.iam.gserviceaccount.com`),
	regexp.MustCompile(`servicenetworking.serviceAgent\/serviceAccount:service-(?:\d*)@service-networking.iam.gserviceaccount.com`),
	regexp.MustCompile(`storage.admin\/serviceAccount:cloud-data-pipeline@koi-b2637a0100e14f34c8c1-tp.iam.gserviceaccount.com`),
	regexp.MustCompile(`storage.admin\/serviceAccount:project-(?:\d*)@storage-transfer-service.iam.gserviceaccount.com`),
	regexp.MustCompile(`storage.objectViewer\/serviceAccount:project-(?:\d*)@storage-transfer-service.iam.gserviceaccount.com`),
	regexp.MustCompile(`vpcaccess.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-vpcaccess.iam.gserviceaccount.com`),
	regexp.MustCompile(`websecurityscanner.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-websecurityscanner.iam.gserviceaccount.com`),
}

func filterDefaultURIs(uris map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for uri := range uris {
		found := false
		for _, re := range defaultURIFilterPatterns {
			matches := re.FindStringSubmatch(uri)
			if len(matches) != 0 {
				found = true
			}
		}
		if !found {
			result[uri] = struct{}{}
		}
	}
	return result
}

// filterIgnored removes any asset iam that is in the ignored assets.
func filterIgnored(values map[string]*iam.AssetIAM, ignored *ignoredAssets) map[string]*iam.AssetIAM {
	filtered := make(map[string]*iam.AssetIAM)
	for k, a := range values {
		if _, ok := ignored.roles[roleURI(a)]; ok {
			continue
		}
		if a.ResourceType == assets.Project {
			if _, ok := ignored.projectIDs[a.ResourceID]; !ok {
				filtered[k] = a
			}
		} else if a.ResourceType == assets.Folder {
			if _, ok := ignored.folderIDs[a.ResourceID]; !ok {
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
	ignoredProjects := ignored.projectIDs
	ignoredFolders := ignored.folderIDs

	// Traverse the hierarchy
	for folderID := range ignored.folderIDs {
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
		iamAssets:  ignored.iamAssets,
		projectIDs: ignoredProjects,
		folderIDs:  ignoredFolders,
	}, nil
}

// driftignore parses the driftignore file into a set.
// Go doesn't implement set so we use a map of boolean values all set to true.
// TODO(dcreey): Consider using yaml/json config https://github.com/abcxyz/guardian/issues/105
func driftignore(
	ctx context.Context,
	fname string,
	gcpFolders map[string]*assets.HierarchyNode,
	gcpProjects map[string]*assets.HierarchyNode,
) (*ignoredAssets, error) {
	logger := logging.FromContext(ctx)
	iamAssets := make(map[string]struct{})
	projects := make(map[string]struct{})
	folders := make(map[string]struct{})
	roles := make(map[string]struct{})
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debugw("failed to find driftignore", "filename", fname)
			return &ignoredAssets{
				iamAssets,
				projects,
				folders,
				roles,
			}, nil
		}
		return nil, fmt.Errorf("failed to read driftignore file %s: %w", fname, err)
	}
	defer f.Close()

	foldersByName := assets.AssetsByName(gcpFolders)
	projectsByName := assets.AssetsByName(gcpProjects)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		iamAssets[line] = struct{}{}

		projectMatches := ignoredProjectPattern.FindStringSubmatch(line)
		if len(projectMatches) == 2 {
			a := projectMatches[1]
			if p, ok := gcpProjects[a]; ok {
				projects[p.ID] = struct{}{}
			} else if p, ok := projectsByName[a]; ok {
				projects[p.ID] = struct{}{}
			} else {
				logger.Warnw("failed to identify ignored project %s", "project", a, "uri", line)
			}
		}

		folderMatches := ignoredFolderPattern.FindStringSubmatch(line)
		if len(folderMatches) == 2 {
			folders[folderMatches[1]] = struct{}{}
			a := folderMatches[1]
			if f, ok := gcpFolders[a]; ok {
				folders[f.ID] = struct{}{}
			} else if f, ok := foldersByName[a]; ok {
				folders[f.ID] = struct{}{}
			} else {
				logger.Warnw("failed to identify ignored folder %s", "folder", a, "uri", line)
			}
		}

		roleMatches := ignoredRolesPattern.FindStringSubmatch(line)
		if len(roleMatches) == 4 {
			roles[roleMatches[0]] = struct{}{}
		}
	}

	return &ignoredAssets{
		iamAssets,
		projects,
		folders,
		roles,
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

func roleURI(a *iam.AssetIAM) string {
	return fmt.Sprintf("roles/%s/%s", a.Role, a.Member)
}
