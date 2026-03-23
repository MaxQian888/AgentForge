package role_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/role"
)

func TestRegistryLoadDirSkipsInvalidFilesAndSupportsLookup(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "reviewer.yaml", validRoleManifest)
	writeRoleFile(t, dir, "planner.yml", stringsReplace(validRoleManifest, "reviewer", "planner"))
	writeRoleFile(t, dir, "invalid.yaml", "metadata: [")
	writeRoleFile(t, dir, "empty-name.yaml", stringsReplace(validRoleManifest, "name: reviewer", "name:"))
	writeRoleFile(t, dir, "notes.txt", validRoleManifest)

	registry := role.NewRegistry()
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if got := registry.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}

	reviewer, ok := registry.Get("reviewer")
	if !ok {
		t.Fatal("Get(reviewer) ok = false, want true")
	}
	if reviewer.Metadata.Description != "Reviews code changes" {
		t.Errorf("reviewer description = %q", reviewer.Metadata.Description)
	}

	names := registry.List()
	slices.Sort(names)
	if !slices.Equal(names, []string{"planner", "reviewer"}) {
		t.Fatalf("List() = %v, want [planner reviewer]", names)
	}

	all := registry.All()
	delete(all, "reviewer")
	if registry.Count() != 2 {
		t.Fatalf("All() should return a copied map; Count() = %d", registry.Count())
	}
}

func TestRegistryRegisterAndLoadDirErrors(t *testing.T) {
	registry := role.NewRegistry()
	registry.Register(&role.Manifest{
		Metadata: role.Metadata{Name: "custom"},
	})

	if got := registry.Count(); got != 1 {
		t.Fatalf("Count() after Register = %d, want 1", got)
	}
	if _, ok := registry.Get("custom"); !ok {
		t.Fatal("Get(custom) ok = false, want true")
	}

	if err := registry.LoadDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("LoadDir() error = nil, want read failure")
	}
}

func writeRoleFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func stringsReplace(input, old, new string) string {
	return strings.NewReplacer(old, new).Replace(input)
}
