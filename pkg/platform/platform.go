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
	"sort"
	"strconv"
	"strings"

	"github.com/posener/complete/v2"

	gh "github.com/abcxyz/guardian/pkg/github"
	"github.com/abcxyz/pkg/cli"
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

// Platform defines the minimum interface for a code review platform.
type Platform interface{}

// Config is the configuration needed to generate different Platform types.
type Config struct {
	Type string

	GitHub gh.Config
}

func (c *Config) RegisterFlags(set *cli.FlagSet) {
	f := set.NewSection("PLATFORM OPTIONS")
	// Type value is loaded in the following order:
	//
	// 1. Explicit value set through --platform flag
	// 2. Inferred environment from well-known environment variables
	// 3. Default value of "local"
	f.StringVar(&cli.StringVar{
		Name:    "platform",
		Target:  &c.Type,
		Example: "github",
		Usage:   fmt.Sprintf("The code review platform for Guardian to integrate with. Allowed values are %q", SortedTypes),
		Predict: complete.PredictFunc(func(prefix string) []string {
			return SortedTypes
		}),
	})

	set.AfterParse(func(merr error) error {
		c.Type = strings.ToLower(strings.TrimSpace(c.Type))

		if _, ok := allowedTypes[c.Type]; !ok && c.Type != TypeUnspecified {
			merr = errors.Join(merr, fmt.Errorf("unsupported value for platform flag: %s", c.Type))
		}

		if c.Type == TypeUnspecified {
			c.Type = TypeLocal
			if v, _ := strconv.ParseBool(set.GetEnv("GITHUB_ACTIONS")); v {
				c.Type = TypeGitHub
			}
		}

		return merr
	})

	c.GitHub.RegisterFlags(set)
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
