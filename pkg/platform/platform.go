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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
)

const (
	TypeLocal  = "local"
	TypeGitHub = "github"
)

var (
	allowedPlatforms = map[string]struct{}{
		TypeLocal:  {},
		TypeGitHub: {},
	}
	_ Platform = (*github.GitHubClient)(nil)
)

// Platform defines the minimum interface for a code review platform.
type Platform interface{}

// Config is the configuration needed to generate different reporter types.
type Config struct {
	Type string

	GitHub *github.Config
}

func (c *Config) RegisterFlags(set *cli.FlagSet) {
	f := set.NewSection("PLATFORM OPTIONS")
	c.GitHub.RegisterFlags(set)

	// Type value is loaded in the following order:
	//
	// 1. Explicit value set through --platform flag
	// 2. Inferred environment from well-known environment variables
	// 3. Default value of "local"
	f.StringVar(&cli.StringVar{
		Name:    "platform",
		Target:  &c.Type,
		Example: "github",
		Usage:   "The code review platform for Guardian to integrate with.",
	})

	set.AfterParse(func(merr error) error {
		c.Type = strings.ToLower(strings.TrimSpace(c.Type))

		if _, ok := allowedPlatforms[c.Type]; !ok && c.Type != "" {
			merr = errors.Join(merr, fmt.Errorf("unsupported value for platform flag: %s", c.Type))
		}

		if c.Type == "" {
			c.Type = "local"
			if v, _ := strconv.ParseBool(set.GetEnv("GITHUB_ACTIONS")); v {
				c.Type = "github"
			}
		}

		return merr
	})
}

// NewPlatform creates a new platform based on the provided type.
func NewPlatform(ctx context.Context, t string, cfg *Config) (Platform, error) {
	if strings.EqualFold(t, TypeLocal) {
		return NewLocal(ctx), nil
	}

	if strings.EqualFold(t, TypeGitHub) {
		gc, err := github.NewGitHubClient(ctx, cfg.GitHub)
		if err != nil {
			return nil, fmt.Errorf("failed to create github client: %w", err)
		}
		return gc, nil
	}

	return nil, fmt.Errorf("unknown platform type: %s", t)
}
