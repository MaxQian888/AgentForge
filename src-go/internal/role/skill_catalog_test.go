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
requires:
  - typescript
tools:
  - code_editor
  - browser-preview
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
	if got := entries[0].Requires; len(got) != 1 || got[0] != "skills/typescript" {
		t.Fatalf("entries[0].Requires = %v, want normalized dependency path", got)
	}
	if got := entries[0].Tools; len(got) != 2 || got[0] != "code_editor" || got[1] != "browser_preview" {
		t.Fatalf("entries[0].Tools = %v, want normalized declared tools", got)
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
	if len(entries[1].Requires) != 0 || len(entries[1].Tools) != 0 {
		t.Fatalf("entries[1] compatibility metadata = %#v, want empty optional metadata", entries[1])
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

func TestDiscoverSkillCatalogReadsStandardSkillParts(t *testing.T) {
	root := t.TempDir()
	mustWriteSkill(t, filepath.Join(root, "design", "SKILL.md"), `# Design`)
	mustWriteSkill(t, filepath.Join(root, "design", "agents", "openai.yaml"), `interface:
  display_name: "Design Expert"
  short_description: "Guide design system decisions."
  default_prompt: "Use $design-expert to review the current design seam."
`)
	mustWriteSkill(t, filepath.Join(root, "design", "references", "surface-map.md"), `# Surface map`)
	mustWriteSkill(t, filepath.Join(root, "design", "scripts", "lint-design.ps1"), `Write-Output "ok"`)
	mustWriteSkill(t, filepath.Join(root, "design", "assets", "token.json"), `{}`)

	entries, err := DiscoverSkillCatalog(root)
	if err != nil {
		t.Fatalf("DiscoverSkillCatalog() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Label != "Design Expert" {
		t.Fatalf("entries[0].Label = %q, want interface display name fallback", entries[0].Label)
	}
	if entries[0].Description != "Guide design system decisions." {
		t.Fatalf("entries[0].Description = %q, want interface short description fallback", entries[0].Description)
	}
	if entries[0].DisplayName != "Design Expert" {
		t.Fatalf("entries[0].DisplayName = %q, want interface display name", entries[0].DisplayName)
	}
	if entries[0].ShortDescription != "Guide design system decisions." {
		t.Fatalf("entries[0].ShortDescription = %q, want interface short description", entries[0].ShortDescription)
	}
	if entries[0].DefaultPrompt == "" {
		t.Fatal("entries[0].DefaultPrompt = empty, want interface prompt")
	}
	if entries[0].ReferenceCount != 1 || entries[0].ScriptCount != 1 || entries[0].AssetCount != 1 {
		t.Fatalf("resource counts = %#v, want 1 each", entries[0])
	}
	if got := entries[0].AvailableParts; len(got) != 4 || got[0] != "agents" || got[1] != "references" || got[2] != "scripts" || got[3] != "assets" {
		t.Fatalf("AvailableParts = %v, want ordered standard parts", got)
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
