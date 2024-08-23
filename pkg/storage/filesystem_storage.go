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

package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FilesystemStorage implements the Storage interface for a local filesystem.
type FilesystemStorage struct {
	parent string
}

// NewFilesystemStorage creates a new FilesystemStorage client.
func NewFilesystemStorage(ctx context.Context, parent string) (*FilesystemStorage, error) {
	return &FilesystemStorage{parent: parent}, nil
}

// CreateObject creates a file in the supplied to the local filesystem..
func (s *FilesystemStorage) CreateObject(ctx context.Context, filename string, contents []byte, _ ...CreateOption) (merr error) {
	pth := filepath.Join(s.parent, filename)
	dir := filepath.Dir(pth)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(pth, contents, 0o600); err != nil {
		return fmt.Errorf("failed to create object: %w", err)
	}
	return nil
}

// Parent returns the filesystem directory.
func (s *FilesystemStorage) Parent() string {
	return s.parent
}

// GetObject returns a reader for a file on the local filesystem. The caller must call Close on the returned Reader when done reading.
func (s *FilesystemStorage) GetObject(ctx context.Context, filename string) (io.ReadCloser, map[string]string, error) {
	pth := filepath.Join(s.parent, filename)
	f, err := os.Open(pth)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	return f, nil, nil
}

// DeleteObject deletes an object from a from the local filesystem. If the object does not exist, no error
// will be returned.
func (s *FilesystemStorage) DeleteObject(ctx context.Context, filename string) error {
	pth := filepath.Join(s.parent, filename)
	if err := os.Remove(pth); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// ObjectsWithName recursively searches a directory for files matching the given filename.
func (s *FilesystemStorage) ObjectsWithName(ctx context.Context, filename string) ([]string, error) {
	var matches []string

	if err := filepath.WalkDir(s.parent, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", pth, err)
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Base(pth) != filename {
			return nil
		}

		matches = append(matches, pth)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to find files: %w", err)
	}

	return matches, nil
}
