package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo holds metadata about a stored file.
type FileInfo struct {
	Key         string
	Size        int64
	ContentType string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

// PutOptions configures file upload behavior.
type PutOptions struct {
	ContentType string
	ExpiresAt   *time.Time
	Metadata    map[string]string
}

// Storage abstracts file storage operations (local filesystem, S3/MinIO).
type Storage interface {
	Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	Exists(ctx context.Context, key string) (bool, error)
}
