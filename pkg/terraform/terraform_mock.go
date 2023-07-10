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
	TerraformResponse
	Err error
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

func (m *MockTerraformClient) Init(ctx context.Context, args ...string) (*TerraformResponse, error) {
	if m.InitResponse != nil {
		return &TerraformResponse{
			Stdout:   m.InitResponse.Stdout,
			Stderr:   m.InitResponse.Stderr,
			ExitCode: m.InitResponse.ExitCode,
		}, m.InitResponse.Err
	}
	return nil, nil
}

func (m *MockTerraformClient) Validate(ctx context.Context, args ...string) (*TerraformResponse, error) {
	if m.ValidateResponse != nil {
		return &TerraformResponse{
			Stdout:   m.ValidateResponse.Stdout,
			Stderr:   m.ValidateResponse.Stderr,
			ExitCode: m.ValidateResponse.ExitCode,
		}, m.ValidateResponse.Err
	}
	return nil, nil
}

func (m *MockTerraformClient) Plan(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	if m.PlanResponse != nil {
		return &TerraformResponse{
			Stdout:   m.PlanResponse.Stdout,
			Stderr:   m.PlanResponse.Stderr,
			ExitCode: m.PlanResponse.ExitCode,
		}, m.PlanResponse.Err
	}
	return nil, nil
}

func (m *MockTerraformClient) Apply(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	if m.ApplyResponse != nil {
		return &TerraformResponse{
			Stdout:   m.ApplyResponse.Stdout,
			Stderr:   m.ApplyResponse.Stderr,
			ExitCode: m.ApplyResponse.ExitCode,
		}, m.ApplyResponse.Err
	}
	return nil, nil
}

func (m *MockTerraformClient) Show(ctx context.Context, file string, args ...string) (*TerraformResponse, error) {
	if m.ShowResponse != nil {
		return &TerraformResponse{
			Stdout:   m.ShowResponse.Stdout,
			Stderr:   m.ShowResponse.Stderr,
			ExitCode: m.ShowResponse.ExitCode,
		}, m.ShowResponse.Err
	}
	return nil, nil
}
