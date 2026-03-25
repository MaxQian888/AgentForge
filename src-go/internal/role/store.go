package role

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

type FileStore struct {
	dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) List() ([]*Manifest, error) {
	registry := NewRegistry()
	if err := s.loadRegistry(registry); err != nil {
		return nil, err
	}

	roles := make([]*Manifest, 0, registry.Count())
	for _, manifest := range registry.All() {
		roles = append(roles, manifest)
	}
	slices.SortFunc(roles, func(a, b *Manifest) int {
		return compareStrings(roleKey(a), roleKey(b))
	})
	return roles, nil
}

func (s *FileStore) Get(id string) (*Manifest, error) {
	registry := NewRegistry()
	if err := s.loadRegistry(registry); err != nil {
		return nil, err
	}

	manifest, ok := registry.Get(id)
	if !ok {
		return nil, os.ErrNotExist
	}
	return manifest, nil
}

func (s *FileStore) Save(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("role manifest is required")
	}

	normalized := cloneManifest(manifest)
	if err := finalizeRoleManifest(normalized); err != nil {
		return err
	}

	roleID := roleKey(normalized)
	roleDir := filepath.Join(s.dir, roleID)
	if err := os.MkdirAll(roleDir, 0o755); err != nil {
		return fmt.Errorf("create role directory %s: %w", roleDir, err)
	}

	data, err := yaml.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal role manifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(roleDir, "role.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("write role manifest: %w", err)
	}

	return nil
}

func (s *FileStore) Delete(id string) error {
	roleDir := filepath.Join(s.dir, id)
	if _, err := os.Stat(roleDir); os.IsNotExist(err) {
		return os.ErrNotExist
	}
	return os.RemoveAll(roleDir)
}

func (s *FileStore) Preview(roleID string, draft *Manifest) (*Manifest, *Manifest, error) {
	registry := NewRegistry()
	if err := s.loadRegistry(registry); err != nil {
		return nil, nil, err
	}

	if draft == nil {
		if roleID == "" {
			return nil, nil, fmt.Errorf("role id or draft is required")
		}
		manifest, ok := registry.Get(roleID)
		if !ok {
			return nil, nil, os.ErrNotExist
		}
		cloned := cloneManifest(manifest)
		return cloned, cloneManifest(cloned), nil
	}

	normalized := cloneManifest(draft)
	if err := finalizeRoleManifest(normalized); err != nil {
		return nil, nil, err
	}

	effective := cloneManifest(normalized)
	if normalized.Extends != "" {
		parent, ok := registry.Get(normalized.Extends)
		if !ok {
			return nil, nil, fmt.Errorf("extended role %s not found", normalized.Extends)
		}
		effective = mergeManifests(parent, normalized)
		if err := finalizeRoleManifest(effective); err != nil {
			return nil, nil, err
		}
	}

	return normalized, effective, nil
}

func (s *FileStore) loadRegistry(registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is required")
	}
	if _, err := os.Stat(s.dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return registry.LoadDir(s.dir)
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
