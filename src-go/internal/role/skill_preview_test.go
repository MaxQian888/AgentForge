package role

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManagedSkillPackagePreviewSupportsRepoAssistantRoots(t *testing.T) {
	repoRoot := t.TempDir()
	skillDir := filepath.Join(repoRoot, ".agents", "skills", "echo-go-backend")
	if err := os.MkdirAll(filepath.Join(skillDir, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatalf("mkdir references: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: Echo Go Backend
description: Build Echo APIs safely.
requires:
  - .agents/skills/next-best-practices
tools:
  - rg
---

# Echo Go Backend

Build Echo APIs safely.
`), 0o644); err != nil {
		t.Fatalf("write skill doc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "agents", "openai.yml"), []byte(`interface:
  display_name: "Echo Go Backend"
  short_description: "Work on Echo backends"
  default_prompt: "Use Echo safely"
`), 0o644); err != nil {
		t.Fatalf("write agent config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "common-task-recipes.md"), []byte("# recipes"), 0o644); err != nil {
		t.Fatalf("write reference: %v", err)
	}

	preview, err := ReadManagedSkillPackagePreview(repoRoot, ".agents/skills/echo-go-backend")
	if err != nil {
		t.Fatalf("ReadManagedSkillPackagePreview() error = %v", err)
	}

	if preview.CanonicalPath != ".agents/skills/echo-go-backend" {
		t.Fatalf("CanonicalPath = %q, want .agents/skills/echo-go-backend", preview.CanonicalPath)
	}
	if preview.Label != "Echo Go Backend" {
		t.Fatalf("Label = %q, want Echo Go Backend", preview.Label)
	}
	if preview.DisplayName != "Echo Go Backend" {
		t.Fatalf("DisplayName = %q, want Echo Go Backend", preview.DisplayName)
	}
	if len(preview.AgentConfigs) != 1 {
		t.Fatalf("len(AgentConfigs) = %d, want 1", len(preview.AgentConfigs))
	}
	if preview.AgentConfigs[0].Path != "agents/openai.yml" {
		t.Fatalf("AgentConfigs[0].Path = %q, want agents/openai.yml", preview.AgentConfigs[0].Path)
	}
	if len(preview.Requires) != 1 || preview.Requires[0] != ".agents/skills/next-best-practices" {
		t.Fatalf("Requires = %#v, want .agents/skills/next-best-practices", preview.Requires)
	}
	if preview.ReferenceCount != 1 {
		t.Fatalf("ReferenceCount = %d, want 1", preview.ReferenceCount)
	}
	if !strings.Contains(preview.FrontmatterYAML, "name: Echo Go Backend") {
		t.Fatalf("FrontmatterYAML = %q, want normalized frontmatter", preview.FrontmatterYAML)
	}
}
