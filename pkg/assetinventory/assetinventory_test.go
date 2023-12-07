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

package assetinventory

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/abcxyz/pkg/testutil"
)

const (
	organizationID = "1231231"
)

var (
	folderA = &HierarchyNode{
		ID:         "123123",
		Name:       "Folder A",
		NodeType:   Organization,
		ParentID:   organizationID,
		ParentType: Organization,
	}
	folderAB = &HierarchyNode{
		ID:         "1231555",
		Name:       "Folder A -> B",
		NodeType:   Folder,
		ParentID:   folderA.ID,
		ParentType: Folder,
	}
	folderABD = &HierarchyNode{
		ID:         "1231555346",
		Name:       "Folder A -> B -> D",
		NodeType:   Folder,
		ParentID:   folderAB.ID,
		ParentType: Folder,
	}
	folderC = &HierarchyNode{
		ID:         "1231666",
		Name:       "Folder C",
		NodeType:   Folder,
		ParentID:   organizationID,
		ParentType: Folder,
	}
	projectA = &HierarchyNode{
		ID:         "1231232222",
		Name:       "Project A",
		NodeType:   Project,
		ParentID:   folderA.ID,
		ParentType: Folder,
	}
	projectAB = &HierarchyNode{
		ID:         "12312322664",
		Name:       "Project AB",
		NodeType:   Project,
		ParentID:   folderAB.ID,
		ParentType: Folder,
	}
	projectRoot = &HierarchyNode{
		ID:         "123123225454",
		Name:       "Project Root",
		NodeType:   Project,
		ParentID:   organizationID,
		ParentType: Organization,
	}
	org = &HierarchyNode{
		ID:         organizationID,
		Name:       "Organization",
		NodeType:   Organization,
		ParentID:   "",
		ParentType: "",
	}
	unknownParentProject = &HierarchyNode{
		ID:         "123123225787",
		Name:       "Unknown Parent Project",
		NodeType:   Project,
		ParentID:   "0000", // Does not exist
		ParentType: Folder,
	}
	unknownParentFolder = &HierarchyNode{
		ID:         "123123225096",
		Name:       "Unknown Parent Folder",
		NodeType:   Folder,
		ParentID:   "0001", // Does not exist
		ParentType: Folder,
	}
	orphanedFolder = &HierarchyNode{
		ID:         "123123225093",
		Name:       "Orhpaned Folder",
		NodeType:   Folder,
		ParentID:   "",
		ParentType: "",
	}
	graph = &HierarchyGraph{
		IDToNodes: map[string]*HierarchyNodeWithChildren{
			organizationID: {
				ProjectIDs:    []string{projectRoot.ID},
				FolderIDs:     []string{folderA.ID, folderC.ID},
				HierarchyNode: org,
			},
			folderA.ID: {
				ProjectIDs:    []string{projectA.ID},
				FolderIDs:     []string{folderAB.ID},
				HierarchyNode: folderA,
			},
			folderAB.ID: {
				ProjectIDs:    []string{projectAB.ID},
				FolderIDs:     []string{folderABD.ID},
				HierarchyNode: folderAB,
			},
			folderABD.ID: {
				ProjectIDs:    []string{},
				FolderIDs:     []string{},
				HierarchyNode: folderABD,
			},
			folderC.ID: {
				ProjectIDs:    []string{},
				FolderIDs:     []string{},
				HierarchyNode: folderC,
			},
		},
	}
)

func TestNewHierarchyGraph(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		orgID         string
		folders       map[string]*HierarchyNode
		projects      map[string]*HierarchyNode
		wantGraph     *HierarchyGraph
		wantErrSubstr string
	}{
		{
			name:      "success",
			orgID:     organizationID,
			folders:   map[string]*HierarchyNode{folderA.ID: folderA, folderAB.ID: folderAB, folderABD.ID: folderABD, folderC.ID: folderC},
			projects:  map[string]*HierarchyNode{projectA.ID: projectA, projectAB.ID: projectAB, projectRoot.ID: projectRoot},
			wantGraph: graph,
		},
		{
			name:          "fails_with_unknown_project_parent",
			orgID:         organizationID,
			folders:       map[string]*HierarchyNode{folderA.ID: folderA, folderAB.ID: folderAB, folderABD.ID: folderABD, folderC.ID: folderC},
			projects:      map[string]*HierarchyNode{projectA.ID: projectA, projectAB.ID: projectAB, projectRoot.ID: projectRoot, unknownParentProject.ID: unknownParentProject},
			wantErrSubstr: fmt.Sprintf("missing reference for folder with ID %s", unknownParentProject.ParentID),
		},
		{
			name:          "fails_with_unknown_folder_parent",
			orgID:         organizationID,
			folders:       map[string]*HierarchyNode{folderA.ID: folderA, folderAB.ID: folderAB, folderABD.ID: folderABD, folderC.ID: folderC, unknownParentFolder.ID: unknownParentFolder},
			projects:      map[string]*HierarchyNode{projectA.ID: projectA, projectAB.ID: projectAB, projectRoot.ID: projectRoot},
			wantErrSubstr: fmt.Sprintf("missing reference for folder with ID %s", unknownParentFolder.ParentID),
		},
		{
			name:          "fails_with_orphaned_folder_parent",
			orgID:         organizationID,
			folders:       map[string]*HierarchyNode{folderA.ID: folderA, folderAB.ID: folderAB, folderABD.ID: folderABD, folderC.ID: folderC, orphanedFolder.ID: orphanedFolder},
			projects:      map[string]*HierarchyNode{projectA.ID: projectA, projectAB.ID: projectAB, projectRoot.ID: projectRoot},
			wantErrSubstr: fmt.Sprintf("missing reference for folder with ID %s", orphanedFolder.ParentID),
		},
	}
	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Run test.
			gotGraph, gotErr := NewHierarchyGraph(tc.orgID, tc.folders, tc.projects)
			if diff := testutil.DiffErrString(gotErr, tc.wantErrSubstr); diff != "" {
				t.Errorf("Process(%+v) got unexpected error substring: %v", tc.name, diff)
			}
			// Verify that the ResourceMapping is modified with additional annotations fetched from Asset Inventory.
			less := func(a, b string) bool { return a < b }
			if diff := cmp.Diff(tc.wantGraph, gotGraph, cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("Process(%+v) got diff (-want, +got): %v", tc.name, diff)
			}
		})
	}
}

func TestFoldersBeneath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		folderID      string
		graph         *HierarchyGraph
		wantFolderIDs map[string]struct{}
		wantErrSubstr string
	}{
		{
			name:     "success",
			folderID: folderAB.ID,
			graph:    graph,
			wantFolderIDs: map[string]struct{}{
				folderABD.ID: {},
			},
		},
		{
			name:     "success_nested",
			folderID: folderA.ID,
			graph:    graph,
			wantFolderIDs: map[string]struct{}{
				folderAB.ID:  {},
				folderABD.ID: {},
			},
		},
		{
			name:          "fails_with_orphaned_folder_parent",
			folderID:      orphanedFolder.ID,
			graph:         graph,
			wantErrSubstr: fmt.Sprintf("missing reference for folder with ID %s", orphanedFolder.ID),
		},
		{
			name:          "fails_with_unknown_folder_parent",
			folderID:      unknownParentFolder.ID,
			graph:         graph,
			wantErrSubstr: fmt.Sprintf("missing reference for folder with ID %s", unknownParentFolder.ID),
		},
	}
	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Run test.
			gotFolderIDs, gotErr := FoldersBeneath(tc.folderID, tc.graph)
			if diff := testutil.DiffErrString(gotErr, tc.wantErrSubstr); diff != "" {
				t.Errorf("Process(%+v) got unexpected error substring: %v", tc.name, diff)
			}
			// Verify that the ResourceMapping is modified with additional annotations fetched from Asset Inventory.
			if diff := cmp.Diff(tc.wantFolderIDs, gotFolderIDs); diff != "" {
				t.Errorf("Process(%+v) got diff (-want, +got): %v", tc.name, diff)
			}
		})
	}
}
