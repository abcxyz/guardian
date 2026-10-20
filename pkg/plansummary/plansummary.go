// Copyright 2026 The Authors (see AUTHORS file)
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

package plansummary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const Filename = "plan-summary.json"

type Summary struct {
	Entrypoint string `json:"entrypoint"`
	Added      int    `json:"added"`
	Changed    int    `json:"changed"`
	Deleted    int    `json:"deleted"`
}

func FromPlanJSON(entrypoint string, planJSON []byte) (*Summary, error) {
	var plan struct {
		ResourceChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	s := &Summary{Entrypoint: filepath.ToSlash(filepath.Clean(entrypoint))}
	for _, rc := range plan.ResourceChanges {
		var isCreate, isDelete, isUpdate bool
		for _, a := range rc.Change.Actions {
			switch a {
			case "create":
				isCreate = true
			case "delete":
				isDelete = true
			case "update":
				isUpdate = true
			}
		}

		switch {
		case isCreate && isDelete:
			s.Added++
			s.Deleted++
		case isCreate:
			s.Added++
		case isDelete:
			s.Deleted++
		case isUpdate:
			s.Changed++
		}
	}
	return s, nil
}

func Write(p string, s *Summary) error {
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal plan summary: %w", err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return fmt.Errorf("failed to write plan summary: %w", err)
	}
	return nil
}

func Read(p string) (*Summary, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan summary: %w", err)
	}
	var s Summary
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("failed to parse plan summary %s: %w", p, err)
	}
	if s.Entrypoint == "" {
		return nil, fmt.Errorf("plan summary %s is missing entrypoint", p)
	}
	s.Entrypoint = filepath.ToSlash(filepath.Clean(s.Entrypoint))
	return &s, nil
}
