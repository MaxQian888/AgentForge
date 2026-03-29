package role

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkillCatalogReturnsNormalizedRepoLocalSkills(t *testing.T) {
	root := t.TempDir()
	mustWriteSkill(t, filepath.Join(root, "react", "SKILL.md"), `---
name: React UI
description: Build React interfaces.
---

# React UI
`)
	mustWriteSkill(t, filepath.Join(root, "testing", "SKILL.md"), `---
description: Verify product behavior.
---

# Testing
`)

	entries, err := DiscoverSkillCatalog(root)
	if err != nil {
		t.Fatalf("DiscoverSkillCatalog() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	if entries[0].Path != "skills/react" {
		t.Fatalf("entries[0].Path = %q, want skills/react", entries[0].Path)
	}
	if entries[0].Label != "React UI" {
		t.Fatalf("entries[0].Label = %q, want frontmatter name", entries[0].Label)
	}
	if entries[0].Description != "Build React interfaces." {
		t.Fatalf("entries[0].Description = %q, want frontmatter description", entries[0].Description)
	}
	if entries[0].Source != "repo-local" || entries[0].SourceRoot != "skills" {
		t.Fatalf("entries[0] source = %#v, want repo-local skills root", entries[0])
	}

	if entries[1].Path != "skills/testing" {
		t.Fatalf("entries[1].Path = %q, want skills/testing", entries[1].Path)
	}
	if entries[1].Label != "Testing" {
		t.Fatalf("entries[1].Label = %q, want path-derived label fallback", entries[1].Label)
	}
	if entries[1].Description != "Verify product behavior." {
		t.Fatalf("entries[1].Description = %q, want description fallback", entries[1].Description)
	}
}

func TestDiscoverSkillCatalogReturnsEmptyForMissingRoot(t *testing.T) {
	entries, err := DiscoverSkillCatalog(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("DiscoverSkillCatalog() error = %v, want nil error for missing root", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0 for missing root", len(entries))
	}
}

func TestDiscoverSkillCatalogSkipsNonSkillFilesAndKeepsFallbackLabel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "README.md"), []byte("not a skill"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	mustWriteSkill(t, filepath.Join(root, "css-animation", "SKILL.md"), `# CSS Animation`)

	entries, err := DiscoverSkillCatalog(root)
	if err != nil {
		t.Fatalf("DiscoverSkillCatalog() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Path != "skills/css-animation" {
		t.Fatalf("entries[0].Path = %q, want normalized path", entries[0].Path)
	}
	if entries[0].Label != "Css Animation" {
		t.Fatalf("entries[0].Label = %q, want path-derived fallback label", entries[0].Label)
	}
	if entries[0].Description != "" {
		t.Fatalf("entries[0].Description = %q, want empty description when metadata missing", entries[0].Description)
	}
}

func mustWriteSkill(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
