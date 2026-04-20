package role_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/role"
)

func TestFileStoreSaveAndListUseCanonicalLayout(t *testing.T) {
	dir := t.TempDir()
	store := role.NewFileStore(dir)

	manifest, err := role.Parse([]byte(canonicalRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := role.ParseFile(filepath.Join(dir, "frontend-developer", "role.yaml")); err != nil {
		t.Fatalf("ParseFile(canonical path) error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "frontend-developer", "role.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "skills:") || !strings.Contains(content, "path: skills/react") {
		t.Fatalf("saved canonical role missing structured skills block:\n%s", content)
	}

	roles, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("List() len = %d, want 1", len(roles))
	}
	if roles[0].Metadata.ID != "frontend-developer" {
		t.Fatalf("List()[0].Metadata.ID = %q, want frontend-developer", roles[0].Metadata.ID)
	}
}

func TestFileStoreSavePreservesAdvancedStructuredFields(t *testing.T) {
	dir := t.TempDir()
	store := role.NewFileStore(dir)

	manifest, err := role.Parse([]byte(advancedRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := role.ParseFile(filepath.Join(dir, "design-lead", "role.yaml"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if got := reloaded.Knowledge.Shared; len(got) != 1 || got[0].ID != "design-guidelines" {
		t.Fatalf("Knowledge.Shared = %#v, want persisted shared knowledge", got)
	}
	if reloaded.Security.Permissions.FileAccess.DeniedPaths[0] != "secrets/" {
		t.Fatalf("DeniedPaths = %v, want persisted denied path", reloaded.Security.Permissions.FileAccess.DeniedPaths)
	}
	if got := reloaded.Triggers; len(got) != 1 || got[0].Event != "pr_created" {
		t.Fatalf("Triggers = %#v, want persisted trigger", got)
	}
}
