// Copyright 2024 The Authors (see AUTHORS file)
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

package platform

import (
	"context"
)

var _ Platform = (*Local)(nil)

// Local implements the Platform interface for running Guardian locally.
type Local struct {
	cfg *localConfig
}

type localConfig struct {
	LocalModifierContent string
}

// NewLocal creates a new Local instance.
func NewLocal(ctx context.Context, cfg *localConfig) *Local {
	return &Local{cfg: cfg}
}

// AssignReviewers is a no-op.
func (l *Local) AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error) {
	return &AssignReviewersResult{}, nil
}

// GetUserRepoPermissions is a no-op and returns an empty string.
func (l *Local) GetUserRepoPermissions(ctx context.Context) (string, error) {
	return "", nil
}

// GetLatestApprovers returns an empty result.
func (l *Local) GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error) {
	return &GetLatestApproversResult{}, nil
}

// GetPolicyData returns an empty result.
func (l *Local) GetPolicyData(ctx context.Context) (*GetPolicyDataResult, error) {
	return &GetPolicyDataResult{}, nil
}

// ModifierContent returns the local modifier content flag or an empty string.
func (l *Local) ModifierContent(ctx context.Context) (string, error) {
	return l.cfg.LocalModifierContent, nil
}

// StoragePrefix returns an empty string for the local platform type.
func (l *Local) StoragePrefix(ctx context.Context) (string, error) {
	return "", nil
}
