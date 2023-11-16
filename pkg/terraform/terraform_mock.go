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
	"io"
)

var _ Terraform = (*MockTerraformClient)(nil)

type MockTerraformResponse struct {
	Stdout   string
	Stderr   string
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
	FormatResponse   *MockTerraformResponse
	RunResponse      *MockTerraformResponse
}

func (m *MockTerraformClient) Init(ctx context.Context, stdout, stderr io.Writer, opts *InitOptions) (int, error) {
	if m.InitResponse != nil {
		stdout.Write([]byte(m.InitResponse.Stdout))
		stderr.Write([]byte(m.InitResponse.Stderr))
		return m.InitResponse.ExitCode, m.InitResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Validate(ctx context.Context, stdout, stderr io.Writer, opts *ValidateOptions) (int, error) {
	if m.ValidateResponse != nil {
		stdout.Write([]byte(m.ValidateResponse.Stdout))
		stderr.Write([]byte(m.ValidateResponse.Stderr))
		return m.ValidateResponse.ExitCode, m.ValidateResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Plan(ctx context.Context, stdout, stderr io.Writer, opts *PlanOptions) (int, error) {
	if m.PlanResponse != nil {
		stdout.Write([]byte(m.PlanResponse.Stdout))
		stderr.Write([]byte(m.PlanResponse.Stderr))
		return m.PlanResponse.ExitCode, m.PlanResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Apply(ctx context.Context, stdout, stderr io.Writer, opts *ApplyOptions) (int, error) {
	if m.ApplyResponse != nil {
		stdout.Write([]byte(m.ApplyResponse.Stdout))
		stderr.Write([]byte(m.ApplyResponse.Stderr))
		return m.ApplyResponse.ExitCode, m.ApplyResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Show(ctx context.Context, stdout, stderr io.Writer, opts *ShowOptions) (int, error) {
	if m.ShowResponse != nil {
		stdout.Write([]byte(m.ShowResponse.Stdout))
		stderr.Write([]byte(m.ShowResponse.Stderr))
		return m.ShowResponse.ExitCode, m.ShowResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Format(ctx context.Context, stdout, stderr io.Writer, opts *FormatOptions) (int, error) {
	if m.FormatResponse != nil {
		stdout.Write([]byte(m.FormatResponse.Stdout))
		stderr.Write([]byte(m.FormatResponse.Stderr))
		return m.FormatResponse.ExitCode, m.FormatResponse.Err
	}
	return 0, nil
}

func (m *MockTerraformClient) Run(ctx context.Context, stdout, stderr io.Writer, subcommand string, args ...string) (int, error) {
	if m.RunResponse != nil {
		stdout.Write([]byte(m.RunResponse.Stdout))
		stderr.Write([]byte(m.RunResponse.Stderr))
		return m.RunResponse.ExitCode, m.RunResponse.Err
	}
	return 0, nil
}
