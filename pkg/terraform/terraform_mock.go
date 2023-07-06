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

package terraform

import (
	"context"
)

var _ Terraform = (*MockTerraformClient)(nil)

type MockTerraformResponse struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

// MockTerraformClient creates a mock TerraformClient for use with testing.
type MockTerraformClient struct {
	InitResponse     *MockTerraformResponse
	ValidateResponse *MockTerraformResponse
	PlanResponse     *MockTerraformResponse
	ApplyResponse    *MockTerraformResponse
	ShowResponse     *MockTerraformResponse

	EntrypointDirs []string
}

func (m *MockTerraformClient) Init(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error) {
	if m.InitResponse != nil {
		return m.InitResponse.Stdout, m.InitResponse.Stderr, m.InitResponse.ExitCode, m.InitResponse.Err
	}
	return []byte{}, []byte{}, 0, nil
}

func (m *MockTerraformClient) Validate(ctx context.Context, workingDir string, args ...string) ([]byte, []byte, int, error) {
	if m.ValidateResponse != nil {
		return m.ValidateResponse.Stdout, m.ValidateResponse.Stderr, m.ValidateResponse.ExitCode, m.ValidateResponse.Err
	}
	return []byte{}, []byte{}, 0, nil
}

func (m *MockTerraformClient) Plan(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	if m.PlanResponse != nil {
		return m.PlanResponse.Stdout, m.PlanResponse.Stderr, m.PlanResponse.ExitCode, m.PlanResponse.Err
	}
	return []byte{}, []byte{}, 0, nil
}

func (m *MockTerraformClient) Apply(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	if m.ApplyResponse != nil {
		return m.ApplyResponse.Stdout, m.ApplyResponse.Stderr, m.ApplyResponse.ExitCode, m.ApplyResponse.Err
	}
	return []byte{}, []byte{}, 0, nil
}

func (m *MockTerraformClient) Show(ctx context.Context, workingDir, file string, args ...string) ([]byte, []byte, int, error) {
	if m.ShowResponse != nil {
		return m.ShowResponse.Stdout, m.ShowResponse.Stderr, m.ShowResponse.ExitCode, m.ShowResponse.Err
	}
	return []byte{}, []byte{}, 0, nil
}

func (m *MockTerraformClient) GetEntrypointDirectories(rootDir string) ([]string, error) {
	if m.EntrypointDirs != nil {
		return m.EntrypointDirs, nil
	}
	return []string{}, nil
}
