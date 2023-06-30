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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
)

var _ Storage = (*GoogleCloudStorage)(nil)

// Config is the configuration for the Google Cloud Storage Client.
type Config struct {
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
	retryMultiplier   float64
	retryTimeout      time.Duration
}

// GoogleCloudStorage implements the Storage interface for Google Cloud Storage.
type GoogleCloudStorage struct {
	client *storage.Client
	cfg    *Config
}

// NewGoogleCloudStorage creates a new GoogleCloudStorage client.
func NewGoogleCloudStorage(ctx context.Context, opts ...Option) (*GoogleCloudStorage, error) {
	cfg := &Config{
		initialRetryDelay: 1 * time.Second,
		maxRetryDelay:     20 * time.Second,
		retryMultiplier:   2,
		retryTimeout:      60 * time.Second,
	}

	for _, opt := range opts {
		if opt != nil {
			cfg = opt(cfg)
		}
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create google cloud storage client: %w", err)
	}

	return &GoogleCloudStorage{
		cfg:    cfg,
		client: client,
	}, nil
}

// getObjectHandleWithRetries retrieves an object handle configured with a retry mechanism.
func (s *GoogleCloudStorage) getObjectHandleWithRetries(ctx context.Context, bucket, name string) (*storage.ObjectHandle, context.Context, context.CancelFunc) {
	o := s.client.Bucket(bucket).Object(name).Retryer(
		storage.WithBackoff(gax.Backoff{
			Initial:    s.cfg.initialRetryDelay,
			Max:        s.cfg.maxRetryDelay,
			Multiplier: s.cfg.retryMultiplier,
		}),
		storage.WithPolicy(storage.RetryAlways),
	)
	ctx, cancel := context.WithTimeout(ctx, s.cfg.retryTimeout)
	return o, ctx, cancel
}

func (s *GoogleCloudStorage) UploadObject(ctx context.Context, bucket, name string, contents []byte, contentType string, metadata map[string]string) error {
	o, ctx, cancel := s.getObjectHandleWithRetries(ctx, bucket, name)
	defer cancel()

	writer := o.NewWriter(ctx)

	if contentType != "" {
		writer.ContentType = contentType
	}

	if metadata != nil {
		writer.Metadata = metadata
	}

	if _, err := writer.Write(contents); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}

func (s *GoogleCloudStorage) GetObject(ctx context.Context, bucket, name string) ([]byte, error) {
	o, ctx, cancel := s.getObjectHandleWithRetries(ctx, bucket, name)
	defer cancel()

	r, err := o.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}

		return nil, fmt.Errorf("failed to get google cloud storage reader: %w", err)
	}
	defer r.Close()

	var data bytes.Buffer
	if _, err := io.Copy(&data, r); err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return data.Bytes(), nil
}

func (s *GoogleCloudStorage) GetObjectWithLimit(ctx context.Context, bucket, name string, limit int64) ([]byte, error) {
	o, ctx, cancel := s.getObjectHandleWithRetries(ctx, bucket, name)
	defer cancel()

	r, err := o.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("failed to get object: %w", err)
		}

		return nil, fmt.Errorf("failed to get google cloud storage reader: %w", err)
	}
	defer r.Close()

	lr := io.LimitReader(r, limit)

	var data bytes.Buffer
	if _, err := io.Copy(&data, lr); err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return data.Bytes(), nil
}

func (s *GoogleCloudStorage) GetMetadata(ctx context.Context, bucket, name string) (map[string]string, error) {
	o, ctx, cancel := s.getObjectHandleWithRetries(ctx, bucket, name)
	defer cancel()

	attrs, err := o.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}
	return attrs.Metadata, nil
}

func (s *GoogleCloudStorage) DeleteObject(ctx context.Context, bucket, name string, ignoreNotFound bool) error {
	o, ctx, cancel := s.getObjectHandleWithRetries(ctx, bucket, name)
	defer cancel()

	if err := o.Delete(ctx); err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) && ignoreNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// Option is an optional config value for the Google Cloud Storage.
type Option func(*Config) *Config

// WithInitialRetryDelay configures the initial delay time before sending a retry for the Google Cloud Storage Client.
func WithInitialRetryDelay(initialRetryDelay time.Duration) Option {
	return func(c *Config) *Config {
		c.initialRetryDelay = initialRetryDelay
		return c
	}
}

// WithMaxRetryDelay configures the maximum delay time before sending a retry for the Google Cloud Storage Client.
func WithMaxRetryDelay(maxRetryDelay time.Duration) Option {
	return func(c *Config) *Config {
		c.maxRetryDelay = maxRetryDelay
		return c
	}
}

// WithRetryMultiplier configures the maximum delay time before sending a retry for the Google Cloud Storage Client.
func WithRetryMultiplier(retryMultiplier float64) Option {
	return func(c *Config) *Config {
		c.retryMultiplier = retryMultiplier
		return c
	}
}

// WithRetryTimeout configures the maximum allowed timeout duration before sending a retry for the Google Cloud Storage Client.
func WithRetryTimeout(retryTimeout time.Duration) Option {
	return func(c *Config) *Config {
		c.retryTimeout = retryTimeout
		return c
	}
}
