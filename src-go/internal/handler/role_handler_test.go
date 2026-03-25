package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/handler"
)

type fakeRoleBridgeClient struct {
	catalog        *bridge.RuntimeCatalogResponse
	catalogErr     error
	generateResult *bridge.GenerateResponse
	generateErr    error
	lastGenerate   *bridge.GenerateRequest
}

func (f *fakeRoleBridgeClient) GetRuntimeCatalog(_ context.Context) (*bridge.RuntimeCatalogResponse, error) {
	return f.catalog, f.catalogErr
}

func (f *fakeRoleBridgeClient) Generate(_ context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error) {
	f.lastGenerate = &req
	return f.generateResult, f.generateErr
}

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

func TestRoleHandlerCreateReturnsAdvancedStructuredFields(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	body := `{
	  "apiVersion": "agentforge/v1",
	  "kind": "Role",
	  "metadata": {
	    "id": "design-lead",
	    "name": "Design Lead",
	    "version": "2.0.0",
	    "icon": "palette"
	  },
	  "identity": {
	    "role": "Design Lead",
	    "goal": "Keep UX coherent",
	    "persona": "Helpful",
	    "responseStyle": {
	      "tone": "professional",
	      "verbosity": "concise",
	      "formatPreference": "markdown"
	    }
	  },
	  "capabilities": {
	    "packages": ["design-system"],
	    "toolConfig": {
	      "builtIn": ["Read"],
	      "external": ["figma"]
	    }
	  },
	  "knowledge": {
	    "shared": [
	      { "id": "design-guidelines", "type": "vector", "access": "read" }
	    ]
	  },
	  "security": {
	    "profile": "standard",
	    "permissionMode": "default",
	    "outputFilters": ["no_pii"]
	  },
	  "collaboration": {
	    "canDelegateTo": ["frontend-developer"]
	  },
	  "triggers": [
	    { "event": "pr_created", "action": "auto_review" }
	  ]
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

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	metadata := created["metadata"].(map[string]any)
	if metadata["icon"] != "palette" {
		t.Fatalf("metadata.icon = %#v, want palette", metadata["icon"])
	}
	knowledge := created["knowledge"].(map[string]any)
	if _, ok := knowledge["shared"]; !ok {
		t.Fatalf("knowledge.shared missing from response: %#v", knowledge)
	}
	security := created["security"].(map[string]any)
	if _, ok := security["outputFilters"]; !ok {
		t.Fatalf("security.outputFilters missing from response: %#v", security)
	}
	if _, ok := created["collaboration"]; !ok {
		t.Fatalf("collaboration missing from response: %#v", created)
	}
	if _, ok := created["triggers"]; !ok {
		t.Fatalf("triggers missing from response: %#v", created)
	}
}

func TestRoleHandlerGetReturnsAdvancedStructuredFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "design-lead"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design-lead", "role.yaml"), []byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: design-lead
  name: Design Lead
  version: "2.0.0"
  icon: palette
identity:
  role: Design Lead
  goal: Keep UX coherent
  persona: Helpful
knowledge:
  shared:
    - id: design-guidelines
      type: vector
      access: read
security:
  profile: standard
  output_filters: [no_pii]
collaboration:
  can_delegate_to: [frontend-developer]
triggers:
  - event: pr_created
    action: auto_review
`), 0o600); err != nil {
		t.Fatalf("seed canonical role error = %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/roles/design-lead", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("design-lead")

	h := handler.NewRoleHandler(dir)
	if err := h.Get(ctx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var rolePayload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rolePayload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	metadata := rolePayload["metadata"].(map[string]any)
	if metadata["icon"] != "palette" {
		t.Fatalf("metadata.icon = %#v, want palette", metadata["icon"])
	}
	knowledge := rolePayload["knowledge"].(map[string]any)
	if _, ok := knowledge["shared"]; !ok {
		t.Fatalf("knowledge.shared missing from response: %#v", knowledge)
	}
	security := rolePayload["security"].(map[string]any)
	if _, ok := security["outputFilters"]; !ok {
		t.Fatalf("security.outputFilters missing from response: %#v", security)
	}
	if _, ok := rolePayload["collaboration"]; !ok {
		t.Fatalf("collaboration missing from response: %#v", rolePayload)
	}
	if _, ok := rolePayload["triggers"]; !ok {
		t.Fatalf("triggers missing from response: %#v", rolePayload)
	}
}

func TestRoleHandlerPreviewReturnsEffectiveManifestForUnsavedDraft(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "base-role"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base-role", "role.yaml"), []byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: base-role
  name: Base Role
identity:
  role: Base Role
  backstory: Parent backstory
capabilities:
  packages: [shared]
security:
  allowed_paths: [app/, components/]
`), 0o600); err != nil {
		t.Fatalf("seed canonical role error = %v", err)
	}

	e := echo.New()
	body := `{
	  "draft": {
	    "apiVersion": "agentforge/v1",
	    "kind": "Role",
	    "metadata": {
	      "id": "child-role",
	      "name": "Child Role"
	    },
	    "extends": "base-role",
	    "identity": {
	      "role": "Child Role",
	      "goal": "Refine the UX"
	    },
	    "capabilities": {
	      "packages": ["review"]
	    },
	    "security": {
	      "allowedPaths": ["app/"]
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/roles/preview", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	h := handler.NewRoleHandler(dir)
	if err := h.Preview(ctx); err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["normalizedManifest"]; !ok {
		t.Fatalf("normalizedManifest missing from response: %#v", payload)
	}
	effective := payload["effectiveManifest"].(map[string]any)
	identity := effective["identity"].(map[string]any)
	if identity["backstory"] != "Parent backstory" {
		t.Fatalf("effective identity.backstory = %#v, want inherited parent value", identity["backstory"])
	}
	capabilities := effective["capabilities"].(map[string]any)
	packages := capabilities["packages"].([]any)
	if len(packages) != 2 {
		t.Fatalf("effective packages = %#v, want inherited + child packages", packages)
	}
}

func TestRoleHandlerSandboxReturnsReadinessDiagnosticsWithoutRunningProbe(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	body := `{
	  "draft": {
	    "apiVersion": "agentforge/v1",
	    "kind": "Role",
	    "metadata": { "id": "sandbox-role", "name": "Sandbox Role" },
	    "identity": { "role": "Sandbox Role", "goal": "Check diagnostics" }
	  },
	  "input": "Review the current role behavior."
	}`
	req := httptest.NewRequest(http.MethodPost, "/roles/sandbox", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	bridgeClient := &fakeRoleBridgeClient{
		catalog: &bridge.RuntimeCatalogResponse{
			DefaultRuntime: "codex",
			Runtimes: []bridge.RuntimeCatalogEntryDTO{
				{
					Key:                 "codex",
					DefaultProvider:     "openai",
					CompatibleProviders: []string{"openai"},
					DefaultModel:        "gpt-5-codex",
					Available:           false,
					Diagnostics: []bridge.RuntimeDiagnosticDTO{
						{Code: "missing_executable", Message: "Codex runtime missing", Blocking: true},
					},
				},
			},
		},
	}

	h := handler.NewRoleHandler(dir).WithBridgeClient(bridgeClient)
	if err := h.Sandbox(ctx); err != nil {
		t.Fatalf("Sandbox() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if bridgeClient.lastGenerate != nil {
		t.Fatalf("Generate() was called despite blocking diagnostics: %#v", bridgeClient.lastGenerate)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	diagnostics := payload["readinessDiagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("readinessDiagnostics = %#v, want one blocking diagnostic", diagnostics)
	}
	if _, ok := payload["probe"]; ok {
		t.Fatalf("probe should be omitted when readiness is blocking: %#v", payload["probe"])
	}
}

func TestRoleHandlerSandboxRunsBoundedProbeWhenRuntimeReady(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	body := `{
	  "draft": {
	    "apiVersion": "agentforge/v1",
	    "kind": "Role",
	    "metadata": { "id": "sandbox-role", "name": "Sandbox Role" },
	    "identity": {
	      "role": "Sandbox Role",
	      "goal": "Review safely",
	      "systemPrompt": "You are a calm reviewer."
	    }
	  },
	  "input": "Summarize the role in one sentence."
	}`
	req := httptest.NewRequest(http.MethodPost, "/roles/sandbox", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	bridgeClient := &fakeRoleBridgeClient{
		catalog: &bridge.RuntimeCatalogResponse{
			DefaultRuntime: "claude_code",
			Runtimes: []bridge.RuntimeCatalogEntryDTO{
				{
					Key:                 "claude_code",
					DefaultProvider:     "anthropic",
					CompatibleProviders: []string{"anthropic"},
					DefaultModel:        "claude-sonnet-4-5",
					Available:           true,
				},
			},
		},
		generateResult: &bridge.GenerateResponse{
			Text: "Calm reviewer focused on safe changes.",
			Usage: bridge.GenerateUsage{
				InputTokens:  20,
				OutputTokens: 12,
			},
		},
	}

	h := handler.NewRoleHandler(dir).WithBridgeClient(bridgeClient)
	if err := h.Sandbox(ctx); err != nil {
		t.Fatalf("Sandbox() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if bridgeClient.lastGenerate == nil {
		t.Fatal("Generate() was not called for ready sandbox probe")
	}
	if bridgeClient.lastGenerate.SystemPrompt != "You are a calm reviewer." {
		t.Fatalf("system prompt = %q, want role prompt", bridgeClient.lastGenerate.SystemPrompt)
	}
	if bridgeClient.lastGenerate.Provider != "anthropic" || bridgeClient.lastGenerate.Model != "claude-sonnet-4-5" {
		t.Fatalf("unexpected probe provider/model: %#v", bridgeClient.lastGenerate)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["probe"]; !ok {
		t.Fatalf("probe missing from response: %#v", payload)
	}
}
