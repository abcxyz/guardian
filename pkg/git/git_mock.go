// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package git

import (
	"context"
)

// MockGitClient implements the git interface.
type MockGitClient struct {
	FetchErr    error
	CheckoutErr error
	DiffResp    []string
	DiffErr     error
	CloneErr    error
}

// Fetch performs a git fetch for given origin and list of refs.
func (m *MockGitClient) Fetch(ctx context.Context, origin string, refs ...string) error {
	return m.FetchErr
}

// Checkout performs a git checkout for a given ref.
func (m *MockGitClient) Checkout(ctx context.Context, ref string) error {
	return m.CheckoutErr
}

// DiffDirsAbs runs a git diff between two revisions and returns the list of directories with changes.
func (m *MockGitClient) DiffDirsAbs(ctx context.Context, baseRef, headRef string) ([]string, error) {
	return m.DiffResp, m.DiffErr
}

// CloneRepository clones the repository to the workingDir.
func (m *MockGitClient) CloneRepository(ctx context.Context, githubToken, owner, repo string) error {
	return m.CloneErr
}
