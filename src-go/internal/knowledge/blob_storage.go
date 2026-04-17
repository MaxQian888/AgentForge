package knowledge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// BlobStorage abstracts file persistence for ingested_file assets.
type BlobStorage interface {
	// Put writes reader contents under a generated key and returns the key.
	Put(ctx context.Context, projectID uuid.UUID, fileName string, r io.Reader) (key string, err error)
	// Get opens a stored blob by key.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes a blob by key.
	Delete(ctx context.Context, key string) error
}

// LocalBlobStorage stores files on the local filesystem under basePath.
type LocalBlobStorage struct {
	basePath string
}

func NewLocalBlobStorage(basePath string) *LocalBlobStorage {
	return &LocalBlobStorage{basePath: basePath}
}

func (s *LocalBlobStorage) Put(ctx context.Context, projectID uuid.UUID, fileName string, r io.Reader) (string, error) {
	if err := os.MkdirAll(filepath.Join(s.basePath, projectID.String()), 0o755); err != nil {
		return "", fmt.Errorf("blob_storage mkdir: %w", err)
	}
	// Build a unique key using a new UUID + original extension.
	ext := filepath.Ext(fileName)
	key := filepath.Join(projectID.String(), uuid.New().String()+ext)
	dst := filepath.Join(s.basePath, key)
	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("blob_storage create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("blob_storage write: %w", err)
	}
	return key, nil
}

func (s *LocalBlobStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(s.basePath, key))
	if err != nil {
		return nil, fmt.Errorf("blob_storage get: %w", err)
	}
	return f, nil
}

func (s *LocalBlobStorage) Delete(ctx context.Context, key string) error {
	if err := os.Remove(filepath.Join(s.basePath, key)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("blob_storage delete: %w", err)
	}
	return nil
}
