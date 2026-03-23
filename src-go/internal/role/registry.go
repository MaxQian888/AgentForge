package role

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Registry holds loaded role manifests indexed by name.
type Registry struct {
	mu    sync.RWMutex
	roles map[string]*Manifest
}

// NewRegistry creates an empty role registry.
func NewRegistry() *Registry {
	return &Registry{
		roles: make(map[string]*Manifest),
	}
}

// LoadDir loads all .yaml and .yml files from a directory into the registry.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read roles dir %s: %w", dir, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		manifest, err := ParseFile(path)
		if err != nil {
			slog.Warn("skipping invalid role file", "path", path, "error", err)
			continue
		}

		if manifest.Metadata.Name == "" {
			slog.Warn("skipping role with empty name", "path", path)
			continue
		}

		r.roles[manifest.Metadata.Name] = manifest
		slog.Info("loaded role", "name", manifest.Metadata.Name, "path", path)
	}

	return nil
}

// Get returns a role manifest by name.
func (r *Registry) Get(name string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.roles[name]
	return m, ok
}

// List returns all loaded role names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.roles))
	for name := range r.roles {
		names = append(names, name)
	}
	return names
}

// All returns all loaded manifests.
func (r *Registry) All() map[string]*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Manifest, len(r.roles))
	for k, v := range r.roles {
		result[k] = v
	}
	return result
}

// Register adds a role manifest to the registry.
func (r *Registry) Register(manifest *Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles[manifest.Metadata.Name] = manifest
}

// Count returns the number of loaded roles.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.roles)
}
