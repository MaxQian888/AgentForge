package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local file storage at the given base directory.
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: basePath}
}

func (s *LocalStorage) Put(_ context.Context, key string, reader io.Reader, _ PutOptions) error {
	fullPath := filepath.Join(s.basePath, key)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

func (s *LocalStorage) Get(_ context.Context, key string) (io.ReadCloser, *FileInfo, error) {
	fullPath := filepath.Join(s.basePath, key)

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	info := &FileInfo{
		Key:       key,
		Size:      stat.Size(),
		CreatedAt: stat.ModTime(),
	}

	return f, info, nil
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.basePath, key))
}

func (s *LocalStorage) List(_ context.Context, prefix string) ([]FileInfo, error) {
	searchDir := filepath.Join(s.basePath, prefix)
	var files []FileInfo

	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(s.basePath, path)
		relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
		files = append(files, FileInfo{
			Key:       relPath,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return files, nil
}

func (s *LocalStorage) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.basePath, key))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
