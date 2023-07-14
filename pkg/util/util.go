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

// Package util contains several utility functions.
package util

import (
	"sort"

	"golang.org/x/exp/maps"
)

// Ptr returns the pointer of a given value.
func Ptr[T any](v T) *T {
	return &v
}

// GetSliceIntersection returns the intersection between two slices.
func GetSliceIntersection(a, b []string) []string {
	intersection := make(map[string]any, 0)

	// Even if slices were 100s or 1000s of records, the performance is negligible.
	// Performance can be improved later if needed
	for _, outer := range a {
		for _, inner := range b {
			if outer == inner {
				intersection[outer] = struct{}{}
				break
			}
		}
	}

	result := maps.Keys(intersection)

	sort.Strings(result)

	return result
}
