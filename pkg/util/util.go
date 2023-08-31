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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ptr returns the pointer of a given value.
func Ptr[T any](v T) *T {
	return &v
}

// ChildPath returns the child path with respect to the base directory
// or returns an error if the target directory is not a child of the base directory.
func ChildPath(base, target string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for target directory %s: %w", target, err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for target directory %s: %w", target, err)
	}

	if strings.TrimSpace(absBase) == strings.TrimSpace(absTarget) {
		return "", nil
	}

	if !strings.HasPrefix(absTarget, absBase) {
		return "", fmt.Errorf("%s is not a child of %s", absTarget, absBase)
	}

	trimmed := strings.TrimPrefix(absTarget, absBase)
	trimmed = strings.TrimPrefix(trimmed, string(os.PathSeparator))

	return trimmed, nil
}

// PathEvalAbs returns the absolute path for a directory after evaluating symlinks.
func PathEvalAbs(path string) (string, error) {
	sym, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err //nolint:wrapcheck // Want passthrough
	}

	abs, err := filepath.Abs(sym)
	if err != nil {
		return "", err //nolint:wrapcheck // Want passthrough
	}

	return abs, nil
}
