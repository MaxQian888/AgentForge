package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
)

func TestRoleHandlerListReadsNormalizedRegistryWithoutPresetFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "frontend-developer"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend-developer.yaml"), []byte(`metadata:
  name: frontend-developer
identity:
  goal: build ui
knowledge:
  system_prompt: hello
`), 0o600); err != nil {
		t.Fatalf("seed legacy role error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend-developer", "role.yaml"), []byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: frontend-developer
  name: Frontend Developer
identity:
  role: Frontend Developer
  goal: build ui
  backstory: build safely
`), 0o600); err != nil {
		t.Fatalf("seed canonical role error = %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	h := handler.NewRoleHandler(dir)
	if err := h.List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var roles []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &roles); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("len(roles) = %d, want 1 without hardcoded preset roles", len(roles))
	}
	metadata := roles[0]["metadata"].(map[string]any)
	if metadata["id"] != "frontend-developer" {
		t.Fatalf("metadata.id = %#v, want frontend-developer", metadata["id"])
	}
	if metadata["name"] != "Frontend Developer" {
		t.Fatalf("metadata.name = %#v, want canonical role name", metadata["name"])
	}
}

func TestRoleHandlerCreatePersistsCanonicalRolePath(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	body := `{
	  "apiVersion": "agentforge/v1",
	  "kind": "Role",
	  "metadata": {
	    "id": "frontend-developer",
	    "name": "Frontend Developer",
	    "version": "1.0.0"
	  },
	  "identity": {
	    "role": "Frontend Developer",
	    "goal": "Build UI",
	    "backstory": "A frontend specialist"
	  },
	  "systemPrompt": "You build safe UI.",
	  "capabilities": {
	    "allowedTools": ["Read", "Edit"],
	    "skills": [
	      { "path": "skills/react", "autoLoad": true },
	      { "path": "skills/testing", "autoLoad": false }
	    ],
	    "maxTurns": 20
	  },
	  "security": {
	    "permissionMode": "default",
	    "allowedPaths": ["app/"]
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/roles", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	h := handler.NewRoleHandler(dir)
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	canonicalPath := filepath.Join(dir, "frontend-developer", "role.yaml")
	if _, err := os.Stat(canonicalPath); err != nil {
		t.Fatalf("canonical role path missing: %v", err)
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	capabilities := created["capabilities"].(map[string]any)
	skills, ok := capabilities["skills"].([]any)
	if !ok || len(skills) != 2 {
		t.Fatalf("response capabilities.skills = %#v, want 2 structured entries", capabilities["skills"])
	}
}

func TestRoleHandlerCreateRejectsDuplicateSkillPaths(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	body := `{
	  "apiVersion": "agentforge/v1",
	  "kind": "Role",
	  "metadata": {
	    "id": "broken-role",
	    "name": "Broken Role",
	    "version": "1.0.0"
	  },
	  "identity": {
	    "role": "Broken Role",
	    "goal": "Break role saving"
	  },
	  "capabilities": {
	    "skills": [
	      { "path": "skills/react", "autoLoad": true },
	      { "path": "skills/react", "autoLoad": false }
	    ]
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/roles", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	h := handler.NewRoleHandler(dir)
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
