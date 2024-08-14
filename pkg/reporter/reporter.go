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

// Package reporter provides an SDK for reporting Guardian results.
package reporter

import (
	"context"
)

// Status is the result of the operation Guardian is performing.
type Status string

// the supported statuses for reporters.
const (
	StatusSuccess     Status = Status("SUCCESS")
	StatusFailure     Status = Status("FAILURE")
	StatusNoOperation Status = Status("NO CHANGES")
	StatusUnknown     Status = Status("UNKNOWN")
)

// Params are the parameters for writing reports.
type Params struct {
	HasDiff   bool
	Details   string
	Dir       string
	IsDestroy bool
	Message   string
	Operation string
}

// Reporter defines the minimum interface for a reporter.
type Reporter interface {
	// CreateStatus reports the status of a run.
	CreateStatus(ctx context.Context, status Status, params *Params) error

	// ClearStatus clears any existing statuses that can be removed.
	ClearStatus(ctx context.Context) error
}
