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

// Package platform defines interfaces for interacting with code review
// platforms.
package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	TypeUnspecified = ""
	TypeLocal       = "local"
	TypeGitHub      = "github"
)

var (
	allowedTypes = map[string]struct{}{
		TypeLocal:  {},
		TypeGitHub: {},
	}
	// SortedTypes are the sorted Platform types for printing messages and prediction.
	SortedTypes = func() []string {
		allowed := append([]string{}, TypeLocal, TypeGitHub)
		sort.Strings(allowed)
		return allowed
	}()
	_ Platform = (*GitHub)(nil)
)

// AssignReviewersInput defines the principal types that can be assigned to a
// change request.
type AssignReviewersInput struct {
	Users []string
	Teams []string
}

// AssignReviewersResult contains the principals that were successfully assigned
// to a change request.
type AssignReviewersResult struct {
	Users []string
	Teams []string
}

// GetLatestApproversResult contains the reviewers whose latest review is an
// approval.
type GetLatestApproversResult struct {
	Users []string `json:"users"`
	Teams []string `json:"teams"`
}

// Platform defines the minimum interface for a code review platform.
type Platform interface {
	// AssignReviewers assigns principals to review a change request.
	AssignReviewers(ctx context.Context, inputs *AssignReviewersInput) (*AssignReviewersResult, error)

	// GetLatestApprovers retrieves the reviewers whose latest review is an
	// approval.
	GetLatestApprovers(ctx context.Context) (*GetLatestApproversResult, error)

	// ModifierContent returns the modifier content for parsing modifier flags.
	ModifierContent(ctx context.Context) string
}

// NewPlatform creates a new platform based on the provided type.
func NewPlatform(ctx context.Context, cfg *Config) (Platform, error) {
	if strings.EqualFold(cfg.Type, TypeLocal) {
		return NewLocal(ctx), nil
	}

	if strings.EqualFold(cfg.Type, TypeGitHub) {
		gc, err := NewGitHub(ctx, &cfg.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github: %w", err)
		}
		return gc, nil
	}

	return nil, fmt.Errorf("unknown platform type: %s", cfg.Type)
}
