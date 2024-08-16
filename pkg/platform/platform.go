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
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
)

const (
	TypeLocal  string = "local"
	TypeGitHub string = "github"
)

var _ Platform = (*github.GitHubClient)(nil)

// Platform defines the minimum interface for a code review platform.
type Platform interface{}

// Config is the configuration needed to generate different reporter types.
type Config struct {
	GitHub github.Config
}

// NewPlatform creates a new platform based on the provided type.
func NewPlatform(ctx context.Context, t string, cfg *Config) (Platform, error) {
	if strings.EqualFold(t, TypeLocal) {
		return NewLocal(ctx), nil
	}

	if strings.EqualFold(t, TypeGitHub) {
		gc, err := github.NewGitHubClient(ctx, &cfg.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}
		return gc, nil
	}

	return nil, fmt.Errorf("unknown platform type: %s", t)
}
