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
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
)

const MiB = 1 << 20 // 1 MiB

var (
	_                 Storage = (*GoogleCloudStorage)(nil)
	ErrBucketNotFound         = errors.New("bucket not found")
)

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
	parent string
}

// NewGoogleCloudStorage creates a new GoogleCloudStorage client.
func NewGoogleCloudStorage(ctx context.Context, parent string) (*GoogleCloudStorage, error) {
	cfg := &Config{
		initialRetryDelay: 1 * time.Second,
		maxRetryDelay:     20 * time.Second,
		retryMultiplier:   2,
		retryTimeout:      60 * time.Second,
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create google cloud storage client: %w", err)
	}

	return &GoogleCloudStorage{
		cfg:    cfg,
		client: client,
		parent: parent,
	}, nil
}

// objectHandleWithRetries retrieves an object handle configured with a retry mechanism.
func (s *GoogleCloudStorage) objectHandleWithRetries(ctx context.Context, name string) (*storage.ObjectHandle, context.Context, context.CancelFunc) {
	o := s.client.Bucket(s.parent).Object(name).Retryer(
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

// makeCreateConfig creates default create config and overwrites with the user
// provided upload options.
func makeCreateConfig(contentLength int, opts []CreateOption) *createConfig {
	defaultChunkSize := 16 * MiB // default for cloud storage client

	// we can send smaller files in one request
	if contentLength <= 16*MiB {
		defaultChunkSize = contentLength + 256
	}

	cfg := &createConfig{
		chunkSize:          defaultChunkSize,
		cacheMaxAgeSeconds: 86400, // 1 day
		allowOverwrite:     false,
	}

	for _, opt := range opts {
		if opt != nil {
			cfg = opt(cfg)
		}
	}

	cacheControl := fmt.Sprintf("public, max-age=%d", cfg.cacheMaxAgeSeconds)
	if cfg.cacheMaxAgeSeconds == 0 {
		cacheControl = "no-cache, max-age=0"
	}
	cfg.cacheControl = cacheControl

	return cfg
}

// CreateObject uploads an object to a Google Cloud Storage bucket using a set of upload options.
func (s *GoogleCloudStorage) CreateObject(ctx context.Context, name string, contents []byte, opts ...CreateOption) (merr error) {
	cfg := makeCreateConfig(len(contents), opts)

	o, ctx, cancel := s.objectHandleWithRetries(ctx, name)
	defer cancel()

	if !cfg.allowOverwrite {
		o = o.If(storage.Conditions{DoesNotExist: true})
	}

	gcsWriter := o.NewWriter(ctx)
	defer func() {
		if closeErr := gcsWriter.Close(); closeErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to close gcs writer: %w", closeErr))
		}
	}()

	gcsWriter.CacheControl = cfg.cacheControl
	gcsWriter.ChunkSize = cfg.chunkSize

	// Per https://cloud.google.com/storage/docs/transcoding#good_practices:
	//
	// When uploading a gzip-compressed object, the recommended way to set your
	// metadata is to specify both the Content-Type and Content-Encoding.
	// Alternatively, you can upload the object with the Content-Type set to
	// indicate compression and NO Content-Encoding at all. In this case the only
	// thing immediately known about the object is that it is gzip-compressed,
	// with no information regarding the underlying object type.
	if cfg.contentType != "" {
		gcsWriter.ContentType = cfg.contentType
		gcsWriter.ContentEncoding = "gzip"
	} else {
		gcsWriter.ContentType = "application/gzip"
	}

	if cfg.metadata != nil {
		m := make(map[string]string, len(cfg.metadata))

		for k, v := range cfg.metadata {
			m[k] = v
		}

		gcsWriter.Metadata = m
	}

	gzipWriter := gzip.NewWriter(gcsWriter)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to close gzip writer: %w", closeErr))
		}
	}()

	if _, err := gzipWriter.Write(contents); err != nil {
		merr = errors.Join(merr, fmt.Errorf("failed to write data: %w", err))
	}

	return merr
}

// GetObject downloads an object from a Google Cloud Storage bucket. The caller must call Close on the returned Reader when done reading.
func (s *GoogleCloudStorage) GetObject(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {
	o, ctx, cancel := s.objectHandleWithRetries(ctx, name)

	r, err := o.NewReader(ctx)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("failed to get google cloud storage reader: %w", err)
	}

	// Get Metadata
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	return &readCloserCanceller{
		ReadCloser: r,
		cancelFunc: cancel,
	}, attrs.Metadata, nil
}

// DeleteObject deletes an object from a Google Cloud Storage bucket. If the object does not exist, no error
// will be returned.
func (s *GoogleCloudStorage) DeleteObject(ctx context.Context, name string) error {
	o, ctx, cancel := s.objectHandleWithRetries(ctx, name)
	defer cancel()

	if err := o.Delete(ctx); err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// ObjectsWithName returns all files in a bucket with a given file name.
func (s *GoogleCloudStorage) ObjectsWithName(ctx context.Context, filename string) ([]string, error) {
	var uris []string
	b := s.client.Bucket(s.parent)
	if _, err := b.Attrs(ctx); err != nil {
		if err.Error() == "storage: bucket doesn't exist" {
			return nil, ErrBucketNotFound
		}
		return nil, fmt.Errorf("failed to stat bucket: %w", err)
	}
	it := b.Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list bucket contents: Bucket(%q).Objects(): %w", s.parent, err)
		}
		if strings.HasSuffix(attrs.Name, filename) {
			uris = append(uris, fmt.Sprintf("gs://%s/%s", s.parent, attrs.Name))
		}
	}
	return uris, nil
}

type readCloserCanceller struct {
	io.ReadCloser
	cancelFunc context.CancelFunc
}

func (r *readCloserCanceller) Close() error {
	defer r.cancelFunc()
	return r.ReadCloser.Close() //nolint:wrapcheck // Want passthrough
}

// SplitObjectURI splits a bucket URI into bucket name and object name or returns an error.
func SplitObjectURI(uri string) (string, string, error) {
	bucketAndObject := strings.SplitN(strings.TrimPrefix(uri, "gs://"), "/", 2)
	if len(bucketAndObject) < 2 {
		return "", "", fmt.Errorf("failed to parse gcs uri: %s", uri)
	}

	return bucketAndObject[0], bucketAndObject[1], nil
}
