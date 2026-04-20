package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/handler"
	skillspkg "github.com/agentforge/server/internal/skills"
	"github.com/labstack/echo/v4"
)

func TestSkillsHandlerListAndGet(t *testing.T) {
	repoRoot := t.TempDir()
	writeSkillsWorkspaceFixture(t, repoRoot)

	h := handler.NewSkillsHandler(skillspkg.NewService(repoRoot))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/skills", nil)
	rec := httptest.NewRecorder()
	if err := h.List(e.NewContext(req, rec)); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}

	var listPayload struct {
		Items []struct {
			ID     string `json:"id"`
			Family string `json:"family"`
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(listPayload.Items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(listPayload.Items))
	}

	req = httptest.NewRequest(http.MethodGet, "/skills/react", nil)
	rec = httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/skills/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues("react")
	if err := h.Get(ctx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", rec.Code)
	}

	var detailPayload struct {
		ID      string `json:"id"`
		Preview struct {
			CanonicalPath string `json:"canonicalPath"`
		} `json:"preview"`
		ConsumerSurfaces []struct {
			ID string `json:"id"`
		} `json:"consumerSurfaces"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("decode detail payload: %v", err)
	}
	if detailPayload.ID != "react" {
		t.Fatalf("id = %q, want react", detailPayload.ID)
	}
	if detailPayload.Preview.CanonicalPath != "skills/react" {
		t.Fatalf("canonicalPath = %q, want skills/react", detailPayload.Preview.CanonicalPath)
	}
	if len(detailPayload.ConsumerSurfaces) == 0 {
		t.Fatal("expected consumer surfaces in detail response")
	}
}

func TestSkillsHandlerVerifyAndSyncMirrors(t *testing.T) {
	repoRoot := t.TempDir()
	writeSkillsWorkspaceFixture(t, repoRoot)

	h := handler.NewSkillsHandler(skillspkg.NewService(repoRoot))
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/skills/verify", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := h.Verify(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", rec.Code)
	}

	var verifyPayload struct {
		OK      bool `json:"ok"`
		Results []struct {
			SkillID string `json:"skillId"`
			Status  string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyPayload); err != nil {
		t.Fatalf("decode verify payload: %v", err)
	}
	if verifyPayload.OK {
		t.Fatal("expected verify ok=false before mirror sync")
	}

	req = httptest.NewRequest(http.MethodPost, "/skills/sync-mirrors", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	if err := h.SyncMirrors(e.NewContext(req, rec)); err != nil {
		t.Fatalf("SyncMirrors() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("sync status = %d, want 200", rec.Code)
	}

	var syncPayload struct {
		UpdatedTargets []string `json:"updatedTargets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &syncPayload); err != nil {
		t.Fatalf("decode sync payload: %v", err)
	}
	if len(syncPayload.UpdatedTargets) != 2 {
		t.Fatalf("len(updatedTargets) = %d, want 2", len(syncPayload.UpdatedTargets))
	}
}

func writeSkillsWorkspaceFixture(t *testing.T, repoRoot string) {
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
---

# React
`)
	mustWrite(filepath.Join(repoRoot, "skills", "react", "agents", "openai.yaml"), `interface:
  display_name: "AgentForge React"
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
