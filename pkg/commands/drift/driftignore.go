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

	"github.com/abcxyz/guardian/pkg/assetinventory"
	"github.com/abcxyz/pkg/logging"
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
// Example: /roles/owner/serviceAccount:platform-ops-tfa-sa-ef3e@platform-ops-ef3e.iam.gserviceaccount.com.
// Example: /roles/resourcemanager.folderEditor/serviceAccount:platform-ops-tfa-sa-ef3e@platform-ops-ef3e.iam.gserviceaccount.com.
var ignoredRolesPattern = regexp.MustCompile(`^\/roles\/([^\/\s]*)\/(serviceAccount|group|user)\:([^\/\s]*)$`)

var defaultURIFilterPatterns = []*regexp.Regexp{
	regexp.MustCompile(`aiplatform\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-aiplatform\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`apigateway\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-apigateway\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`apigateway_management\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-apigateway-mgmt\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`appengine\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-gae-service\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`appengineflex\.serviceAgent\/serviceAccount:service-(?:\d*)@gae-api-prod\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`appengineflex\.serviceAgent\/serviceAccount:service-(?:\d*)@gae-api-prod\.google\.com\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`artifactregistry\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-artifactregistry\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`batch\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudbatch\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`bigquerydatatransfer\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-bigquerydatatransfer\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`binaryauthorization\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-binaryauthorization\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudasset\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudasset\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudbuild\.builds\.builder\/serviceAccount:(?:\d*)@cloudbuild\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudbuild\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudbuild\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudfunctions\.serviceAgent\/serviceAccount:service-(?:\d*)@gcf-admin-robot\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudfunctions\.serviceAgent\/serviceAccount:service-project-(?:\d*)@security-center-api\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudiot\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudiot\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudkms\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudkms\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudscheduler\.serviceAgent\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudscheduler\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudscheduler\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`cloudtpu\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-tpu\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`compute\.networkViewer\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`compute\.serviceAgent\/serviceAccount:service-(?:\d*)@compute-system\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`connectors\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-connectors\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`container\.serviceAgent\/serviceAccount:service-(?:\d*)@container-engine-robot\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`containeranalysis\.ServiceAgent\/serviceAccount:service-(?:\d*)@container-analysis\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`containerregistry\.ServiceAgent\/serviceAccount:service-(?:\d*)@containerregistry\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`containerscanning\.ServiceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-containerscanning\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`containerthreatdetection.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-ktd-control\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`dataflow\.serviceAgent\/serviceAccount:service-(?:\d*)@dataflow-service-producer-prod\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`dataform\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-dataform\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`datafusion\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-datafusion\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`datapipelines\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-datapipelines\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`dataprep\.serviceAgent\/serviceAccount:service-(?:\d*)@trifacta-gcloud-prod\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`dataproc\.serviceAgent\/serviceAccount:service-(?:\d*)@dataproc-accounts\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`editor\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`editor\/serviceAccount:(?:\d*)@cloudservices\.gserviceaccount\.com`),
	regexp.MustCompile(`endpointsportal\.serviceAgent\/serviceAccount:service-(?:\d*)@endpoints-portal\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`eventarc\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-eventarc\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`file\.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-filer\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`firebase\.managementServiceAgent\/serviceAccount:firebase-service-account@firebase-sa-management\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`firebase\.managementServiceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-firebase\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`firebaserules\.system\/serviceAccount:service-(?:\d*)@firebase-rules\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`firebasestorage\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-firebasestorage\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`firestore\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-firestore\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`healthcare\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-healthcare\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`iap\.settingsAdmin\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`identitytoolkit\.viewer\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`integrations\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-integrations\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`lifesciences\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-lifesciences\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`logging\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-logging\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`ml\.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-ml\.google\.com\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`ml\.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-ml\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`monitoring\.notificationServiceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-monitoring-notification\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`networkmanagement\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-networkmanagement\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`notebooks\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-notebooks\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`osconfig\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-osconfig\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`pubsub\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-pubsub\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`redis\.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-redis\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`run\.serviceAgent\/serviceAccount:service-(?:\d*)@serverless-robot-prod\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`securitycenter\.serviceAgent\/serviceAccount:service-org-(?:\d*)@security-center-api\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`securitycenter\.serviceAgent\/serviceAccount:service-project-(?:\d*)@security-center-api\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`servicenetworking\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-cloudasset\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`servicenetworking\.serviceAgent\/serviceAccount:service-(?:\d*)@service-networking\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`spanner\.serviceAgent\/serviceAccount:(?:\d*)-compute@developer\.gserviceaccount\.com`),
	regexp.MustCompile(`storage\.admin\/serviceAccount:cloud-data-pipeline@koi-b2637a0100e14f34c8c1-tp\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`storage\.admin\/serviceAccount:project-(?:\d*)@storage-transfer-service\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`storage\.objectViewer\/serviceAccount:project-(?:\d*)@storage-transfer-service\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`storageinsights\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-storageinsights\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`storagetransfer\.serviceAgent\/serviceAccount:project-(?:\d*)@storage-transfer-service\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`tpu\.serviceAgent\/serviceAccount:service-(?:\d*)@cloud-tpu\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`vpcaccess\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-vpcaccess\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`websecurityscanner\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-websecurityscanner\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`workflows\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-workflows\.iam\.gserviceaccount\.com`),
	regexp.MustCompile(`workstations\.serviceAgent\/serviceAccount:service-(?:\d*)@gcp-sa-workstations\.iam\.gserviceaccount\.com`),
}

func filterDefaultURIs(uris []string) []string {
	result := []string{}
	for _, uri := range uris {
		found := false
		for _, re := range defaultURIFilterPatterns {
			matches := re.FindStringSubmatch(uri)
			if len(matches) != 0 {
				found = true
			}
		}
		if !found {
			result = append(result, uri)
		}
	}
	return result
}

// filterIgnored removes any asset iam that is in the ignored assets.
func filterIgnored(values map[string]*assetinventory.AssetIAM, ignored *ignoredAssets) map[string]*assetinventory.AssetIAM {
	filtered := make(map[string]*assetinventory.AssetIAM)
	for k, a := range values {
		if !isIgnored(a, ignored) {
			filtered[k] = a
		}
	}
	return filtered
}

// filterIgnoredTF removes any tf asset iam that is in the ignored assets.
func filterIgnoredTF(values map[string]*TerraformStateIAMSource, ignored *ignoredAssets) map[string]*TerraformStateIAMSource {
	filtered := make(map[string]*TerraformStateIAMSource)
	for k, a := range values {
		if !isIgnored(a.AssetIAM, ignored) {
			filtered[k] = a
		}
	}
	return filtered
}

func isIgnored(a *assetinventory.AssetIAM, ignored *ignoredAssets) bool {
	if _, ok := ignored.roles[roleURI(a)]; ok {
		return true
	}
	switch a.ResourceType {
	case assetinventory.Project:
		if _, ok := ignored.projectIDs[a.ResourceID]; !ok {
			return false
		}
		return true
	case assetinventory.Folder:
		if _, ok := ignored.folderIDs[a.ResourceID]; !ok {
			return false
		}
		return true
	default:
		// Handle default case so we do not accidentally drop values.
		return false
	}
}

func selectFrom[K any](values []string, from map[string]K) map[string]K {
	r := make(map[string]K, len(values))
	for _, v := range values {
		if f, ok := from[v]; ok {
			r[v] = f
		}
	}
	return r
}

// expandGraph traverses the asset hierarchy graph and adds any nested folders or projects beneath every ignored asset.
func expandGraph(ignored *ignoredAssets, hierarchyGraph *assetinventory.HierarchyGraph) (*ignoredAssets, error) {
	ignoredProjects := ignored.projectIDs
	ignoredFolders := ignored.folderIDs

	// Traverse the hierarchy
	for folderID := range ignored.folderIDs {
		ids, err := assetinventory.FoldersBeneath(folderID, hierarchyGraph)
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
		roles:      ignored.roles,
	}, nil
}

// driftignore parses the driftignore file into a set.
// Go doesn't implement set so we use a map of boolean values all set to true.
// TODO(dcreey): Consider using yaml/json config https://github.com/abcxyz/guardian/issues/105
func driftignore(
	ctx context.Context,
	fname string,
	gcpFolders map[string]*assetinventory.HierarchyNode,
	gcpProjects map[string]*assetinventory.HierarchyNode,
	deletedGCPFolders map[string]*assetinventory.HierarchyNode,
	deletedGCPProjects map[string]*assetinventory.HierarchyNode,
) (*ignoredAssets, error) {
	logger := logging.FromContext(ctx)
	iamAssets := make(map[string]struct{})
	projects := make(map[string]struct{})
	folders := make(map[string]struct{})
	roles := make(map[string]struct{})

	for id := range deletedGCPFolders {
		folders[id] = struct{}{}
	}
	for id := range deletedGCPProjects {
		projects[id] = struct{}{}
	}

	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			logger.DebugContext(ctx, "failed to find driftignore", "filename", fname)
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

	foldersByName := assetinventory.AssetsByName(gcpFolders)
	projectsByName := assetinventory.AssetsByName(gcpProjects)

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
				logger.WarnContext(ctx, "failed to identify ignored project",
					"project", a,
					"uri", line)
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
				logger.WarnContext(ctx, "failed to identify ignored folder",
					"folder", a,
					"uri", line)
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

func roleURI(a *assetinventory.AssetIAM) string {
	return fmt.Sprintf("/%s/%s", a.Role, a.Member)
}
