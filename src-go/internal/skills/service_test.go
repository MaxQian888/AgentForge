package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceListReturnsGovernedSkillInventory(t *testing.T) {
	repoRoot := t.TempDir()
	writeGovernedSkillFixture(t, repoRoot)

	svc := NewService(repoRoot)
	items, err := svc.List(ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}

	byID := make(map[string]InventoryItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}

	react, ok := byID["react"]
	if !ok {
		t.Fatalf("missing react inventory item: %+v", items)
	}
	if react.Family != FamilyBuiltInRuntime {
		t.Fatalf("react family = %q, want %q", react.Family, FamilyBuiltInRuntime)
	}
	if !react.Bundle.Member {
		t.Fatal("expected react to be part of built-in bundle")
	}
	if react.Health.Status != HealthHealthy {
		t.Fatalf("react health = %q, want %q", react.Health.Status, HealthHealthy)
	}
	if len(react.ConsumerSurfaces) == 0 {
		t.Fatal("expected react consumer surfaces to be resolved")
	}

	shadcn, ok := byID["shadcn"]
	if !ok {
		t.Fatalf("missing shadcn inventory item: %+v", items)
	}
	if shadcn.Family != FamilyRepoAssistant {
		t.Fatalf("shadcn family = %q, want %q", shadcn.Family, FamilyRepoAssistant)
	}
	if shadcn.Lock == nil || shadcn.Lock.Key != "shadcn" {
		t.Fatalf("shadcn lock = %#v, want lockKey shadcn", shadcn.Lock)
	}
	if shadcn.Health.Status != HealthHealthy {
		t.Fatalf("shadcn health = %q, want %q", shadcn.Health.Status, HealthHealthy)
	}

	workflow, ok := byID["openspec-propose"]
	if !ok {
		t.Fatalf("missing openspec-propose inventory item: %+v", items)
	}
	if workflow.Family != FamilyWorkflowMirror {
		t.Fatalf("workflow family = %q, want %q", workflow.Family, FamilyWorkflowMirror)
	}
	if len(workflow.MirrorTargets) != 2 {
		t.Fatalf("len(MirrorTargets) = %d, want 2", len(workflow.MirrorTargets))
	}
	if workflow.Health.Status != HealthDrifted {
		t.Fatalf("workflow health = %q, want %q due to mirror drift", workflow.Health.Status, HealthDrifted)
	}
}

func TestServiceVerifyAndSyncMirrorsReportDiagnostics(t *testing.T) {
	repoRoot := t.TempDir()
	writeGovernedSkillFixture(t, repoRoot)

	svc := NewService(repoRoot)

	verifyResult, err := svc.Verify(VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verifyResult.OK {
		t.Fatal("Verify() ok = true, want false before mirror sync")
	}
	if len(verifyResult.Results) == 0 {
		t.Fatal("expected per-skill verification results")
	}

	syncResult, err := svc.SyncMirrors(SyncMirrorsOptions{})
	if err != nil {
		t.Fatalf("SyncMirrors() error = %v", err)
	}
	if len(syncResult.UpdatedTargets) != 2 {
		t.Fatalf("len(UpdatedTargets) = %d, want 2", len(syncResult.UpdatedTargets))
	}

	verifyAfterSync, err := svc.Verify(VerifyOptions{Families: []Family{FamilyWorkflowMirror}})
	if err != nil {
		t.Fatalf("Verify() after sync error = %v", err)
	}
	if !verifyAfterSync.OK {
		t.Fatalf("Verify() after sync ok = false, want true; results=%+v", verifyAfterSync.Results)
	}
}

func writeGovernedSkillFixture(t *testing.T, repoRoot string) {
	t.Helper()

	mustWrite := func(path string, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	mustWrite(filepath.Join(repoRoot, "internal-skills.yaml"), `version: 1
skills:
  - id: react
    family: built-in-runtime
    verificationProfile: built-in-runtime
    canonicalRoot: skills/react
    sourceType: repo-authored
    docsRef: docs/role-yaml.md
  - id: shadcn
    family: repo-assistant
    verificationProfile: repo-assistant
    canonicalRoot: .agents/skills/shadcn
    sourceType: upstream-sync
    lockKey: shadcn
    allowedExceptions:
      - noncanonical-agent-config-extension
  - id: openspec-propose
    family: workflow-mirror
    verificationProfile: workflow-mirror
    canonicalRoot: .codex/skills/openspec-propose
    sourceType: repo-authored
    mirrorTargets:
      - .claude/skills/openspec-propose/SKILL.md
      - .github/skills/openspec-propose/SKILL.md
`)
	mustWrite(filepath.Join(repoRoot, "skills-lock.json"), `{
  "version": 1,
  "skills": {
    "shadcn": {
      "source": "shadcn/ui",
      "sourceType": "github",
      "computedHash": "demo-hash"
    }
  }
}`)
	mustWrite(filepath.Join(repoRoot, "skills", "builtin-bundle.yaml"), `skills:
  - id: react
    root: react
    category: frontend
    tags:
      - react
      - nextjs
    docsRef: docs/role-yaml.md
    featured: true
`)

	mustWrite(filepath.Join(repoRoot, "skills", "react", "SKILL.md"), `---
name: React
description: Build React surfaces.
requires:
  - skills/typescript
tools:
  - browser_preview
---

# React
`)
	mustWrite(filepath.Join(repoRoot, "skills", "react", "agents", "openai.yaml"), `interface:
  display_name: "AgentForge React"
  short_description: "Build React safely"
`)
	mustWrite(filepath.Join(repoRoot, ".agents", "skills", "shadcn", "SKILL.md"), `---
name: shadcn
description: UI component skill
---

# shadcn
`)
	mustWrite(filepath.Join(repoRoot, ".agents", "skills", "shadcn", "agents", "openai.yml"), `interface:
  display_name: "shadcn/ui"
`)
	mustWrite(filepath.Join(repoRoot, ".codex", "skills", "openspec-propose", "SKILL.md"), `---
name: openspec-propose
description: Propose OpenSpec changes
---

# openspec-propose
`)
	mustWrite(filepath.Join(repoRoot, ".claude", "skills", "openspec-propose", "SKILL.md"), `stale mirror`)
	mustWrite(filepath.Join(repoRoot, ".github", "skills", "openspec-propose", "SKILL.md"), `stale mirror`)
}
