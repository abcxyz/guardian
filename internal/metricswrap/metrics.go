// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricswrap

import (
	"context"
	"time"

	"github.com/abcxyz/abc-updater/pkg/metrics"
	"github.com/abcxyz/pkg/logging"
)

// A little on the long side to tolerate cold starts. Metrics run concurrently
// so users will see at most 500ms runtime from this in most cases.
const defaultMetricsTimeout = 500 * time.Millisecond

// WriteMetric is an async wrapper for metrics.WriteMetric.
// It returns a function that blocks on completion and handles any errors.
func WriteMetric(ctx context.Context, client *metrics.Client, name string, count int64) func() {
	ctx, done := context.WithTimeout(ctx, defaultMetricsTimeout)
	errCh := make(chan error, 1)
	go func() {
		defer done()
		defer close(errCh)
		errCh <- client.WriteMetric(ctx, name, count)
	}()

	return func() {
		err := <-errCh
		if err != nil {
			logger := logging.FromContext(ctx)
			logger.DebugContext(ctx, "Metric writing failed.", "err", err)
		}
	}
}
