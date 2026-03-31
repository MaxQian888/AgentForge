package role_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryRolesDirectoryDoesNotContainLegacyDuplicates(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	rolesDir := filepath.Join(repoRoot, "roles")

	entries, err := os.ReadDir(rolesDir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", rolesDir, err)
	}

	var duplicates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		roleID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		canonicalPath := filepath.Join(rolesDir, roleID, "role.yaml")
		if _, err := os.Stat(canonicalPath); err == nil {
			duplicates = append(duplicates, entry.Name())
		}
	}

	slices.Sort(duplicates)
	if len(duplicates) > 0 {
		t.Fatalf("roles directory contains legacy duplicates alongside canonical manifests: %v", duplicates)
	}
}
