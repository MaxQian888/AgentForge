package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
)

type handlerRuntimeClient struct{}

type handlerGoRuntime struct {
	result map[string]any
}

type handlerRoleStore struct {
	roles map[string]*rolepkg.Manifest
}

type handlerWorkflowExecutor struct {
	calls []service.WorkflowStepExecutionRequest
}

func (handlerRuntimeClient) RegisterToolPlugin(_ context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       manifest.Metadata.ID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateInstalled,
	}, nil
}

func (handlerRuntimeClient) ActivateToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (handlerRuntimeClient) CheckToolPluginHealth(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (handlerRuntimeClient) RestartToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
		RestartCount:   1,
	}, nil
}

func (h *handlerGoRuntime) ActivatePlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       record.Metadata.ID,
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateActive,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
	}, nil
}

func (h *handlerGoRuntime) CheckPluginHealth(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       record.Metadata.ID,
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateActive,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
	}, nil
}

func (h *handlerGoRuntime) RestartPlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       record.Metadata.ID,
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateActive,
		RestartCount:   1,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
	}, nil
}

func (h *handlerGoRuntime) Invoke(_ context.Context, _ model.PluginRecord, _ string, _ map[string]any) (map[string]any, error) {
	if h.result == nil {
		h.result = map[string]any{"status": "ok"}
	}
	return h.result, nil
}

func (s *handlerRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if s == nil || s.roles == nil {
		return nil, os.ErrNotExist
	}
	role, ok := s.roles[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return role, nil
}

func (h *handlerWorkflowExecutor) Execute(_ context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	h.calls = append(h.calls, req)
	return &service.WorkflowStepExecutionResult{
		Output: map[string]any{
			"step":   req.Step.ID,
			"status": "ok",
		},
	}, nil
}

func writePluginManifest(t *testing.T, dir string, relative string, content string) string {
	t.Helper()
	path := filepath.Join(dir, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func newPluginHandler(t *testing.T, pluginsDir string) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), handlerRuntimeClient{}, nil, pluginsDir)
	return handler.NewPluginHandler(svc)
}

func newPluginHandlerWithRegistry(t *testing.T, repo service.PluginRegistry, pluginsDir string) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repo, handlerRuntimeClient{}, nil, pluginsDir)
	return handler.NewPluginHandler(svc)
}

func newPluginHandlerWithGoRuntime(t *testing.T, pluginsDir string, goRuntime service.GoPluginRuntime) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), handlerRuntimeClient{}, goRuntime, pluginsDir)
	return handler.NewPluginHandler(svc)
}

func newPluginHandlerWithWorkflowDeps(
	t *testing.T,
	repo service.PluginRegistry,
	pluginsDir string,
	goRuntime service.GoPluginRuntime,
	roleStore service.PluginRoleStore,
) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repo, handlerRuntimeClient{}, goRuntime, pluginsDir).WithRoleStore(roleStore)
	return handler.NewPluginHandler(svc)
}

func newPluginHandlerWithWorkflowRuntime(
	t *testing.T,
	repo service.PluginRegistry,
	pluginsDir string,
	goRuntime service.GoPluginRuntime,
	roleStore service.PluginRoleStore,
	executor service.WorkflowStepExecutor,
) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repo, handlerRuntimeClient{}, goRuntime, pluginsDir).WithRoleStore(roleStore)
	workflowSvc := service.NewWorkflowExecutionService(svc, repository.NewWorkflowPluginRunRepository(), roleStore, executor)
	return handler.NewPluginHandler(svc).WithWorkflowExecution(workflowSvc)
}

func TestPluginHandler_InstallLocalAndList(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/repo-search.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: repo-search
  name: Repo Search
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)

	h := newPluginHandler(t, pluginsDir)
	e := echo.New()

	body, _ := json.Marshal(map[string]string{"path": manifestPath})
	req := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.InstallLocal(c); err != nil {
		t.Fatalf("install local: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/plugins?kind=ToolPlugin", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := h.List(listCtx); err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}

	var records []model.PluginRecord
	if err := json.Unmarshal(listRec.Body.Bytes(), &records); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(records) != 1 || records[0].Metadata.ID != "repo-search" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func (handlerRuntimeClient) RefreshToolPluginMCPSurface(_ context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	return &model.PluginMCPRefreshResult{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostTSBridge,
		Snapshot: model.PluginMCPCapabilitySnapshot{
			Transport: "stdio",
		},
	}, nil
}

func (handlerRuntimeClient) InvokeToolPluginMCPTool(_ context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionCallTool),
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: toolName}},
		},
	}, nil
}

func (handlerRuntimeClient) ReadToolPluginMCPResource(_ context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	return &model.PluginMCPResourceReadResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionReadResource),
		Result: model.MCPResourceReadResult{
			Contents: []model.MCPResourceContent{{URI: uri}},
		},
	}, nil
}

func (handlerRuntimeClient) GetToolPluginMCPPrompt(_ context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	return &model.PluginMCPPromptResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionGetPrompt),
		Result: model.MCPPromptGetResult{
			Description: name,
		},
	}, nil
}

func TestPluginHandler_RuntimeStateSync(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: feishu
  name: Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)

	h := newPluginHandler(t, pluginsDir)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := h.InstallLocal(installCtx); err != nil {
		t.Fatalf("install local: %v", err)
	}

	updateBody, _ := json.Marshal(model.PluginRuntimeStatus{
		PluginID:       "feishu",
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateDegraded,
		LastError:      "health check failed",
		RestartCount:   3,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/internal/plugins/runtime-state", bytes.NewReader(updateBody))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)

	if err := h.SyncRuntimeState(updateCtx); err != nil {
		t.Fatalf("sync runtime state: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateRec.Code)
	}

	var record model.PluginRecord
	if err := json.Unmarshal(updateRec.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	if record.LifecycleState != model.PluginStateDegraded {
		t.Fatalf("expected degraded state, got %s", record.LifecycleState)
	}
	if record.RestartCount != 3 {
		t.Fatalf("expected restart count 3, got %d", record.RestartCount)
	}
}

func TestPluginHandler_InvokeIntegrationPlugin(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: feishu
  name: Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
  capabilities: ["send_message"]
`)

	goRuntime := &handlerGoRuntime{
		result: map[string]any{
			"status": "sent",
		},
	}
	h := newPluginHandlerWithGoRuntime(t, pluginsDir, goRuntime)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := h.InstallLocal(installCtx); err != nil {
		t.Fatalf("install local: %v", err)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/feishu/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("feishu")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"operation": "send_message",
		"payload": map[string]any{
			"chat_id": "chat-1",
			"content": "hello",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/plugins/feishu/invoke", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("feishu")

	if err := h.Invoke(c); err != nil {
		t.Fatalf("invoke plugin: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		PluginID  string         `json:"plugin_id"`
		Operation string         `json:"operation"`
		Result    map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode invoke response: %v", err)
	}
	if response.PluginID != "feishu" || response.Operation != "send_message" {
		t.Fatalf("unexpected invoke response header: %+v", response)
	}
	if response.Result["status"] != "sent" {
		t.Fatalf("expected sent result, got %+v", response.Result)
	}
}

func TestPluginHandler_MarketplaceReturnsManifestBackedCatalog(t *testing.T) {
	pluginsDir := t.TempDir()
	writePluginManifest(t, pluginsDir, "integrations/feishu/manifest.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: feishu
  name: Feishu
  version: 1.0.0
  description: Built-in Feishu adapter
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)

	h := newPluginHandler(t, pluginsDir)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/plugins/marketplace", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Marketplace(c); err != nil {
		t.Fatalf("marketplace: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var plugins []model.MarketplacePluginDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &plugins); err != nil {
		t.Fatalf("decode marketplace response: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(plugins))
	}
	if plugins[0].ID != "feishu" {
		t.Fatalf("catalog id = %q, want feishu", plugins[0].ID)
	}
	if plugins[0].InstallURL == "" {
		t.Fatal("expected install url to be populated from the manifest path")
	}
}

func TestPluginHandler_ListEventsReturnsAuditTrail(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: feishu
  name: Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)

	h := newPluginHandlerWithGoRuntime(t, pluginsDir, &handlerGoRuntime{})
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := h.InstallLocal(installCtx); err != nil {
		t.Fatalf("install local: %v", err)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/feishu/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("feishu")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/plugins/feishu/events", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("feishu")

	if err := h.ListEvents(c); err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var events []model.PluginEventRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode events response: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected non-empty plugin event list")
	}
	if events[0].PluginID != "feishu" {
		t.Fatalf("event plugin id = %q, want feishu", events[0].PluginID)
	}
}

func TestPluginHandler_ListSupportsSourceAndTrustFilters(t *testing.T) {
	pluginsDir := t.TempDir()
	repo := repository.NewPluginRegistryRepository()
	ctx := context.Background()

	records := []*model.PluginRecord{
		{
			PluginManifest: model.PluginManifest{
				APIVersion: "agentforge/v1",
				Kind:       model.PluginKindReview,
				Metadata: model.PluginMetadata{
					ID:      "review.typescript",
					Name:    "TypeScript Review",
					Version: "1.0.0",
				},
				Spec: model.PluginSpec{
					Runtime: model.PluginRuntimeMCP,
					Review: &model.ReviewPluginSpec{
						Entrypoint: "review:run",
						Triggers: model.ReviewPluginTrigger{
							Events: []string{"pull_request.updated"},
						},
						Output: model.ReviewPluginOutput{Format: "findings/v1"},
					},
				},
				Source: model.PluginSource{
					Type:    model.PluginSourceNPM,
					Package: "@agentforge/review-typescript",
					Trust: &model.PluginTrustMetadata{
						Status: model.PluginTrustVerified,
					},
				},
			},
			LifecycleState: model.PluginStateEnabled,
			RuntimeHost:    model.PluginHostTSBridge,
		},
		{
			PluginManifest: model.PluginManifest{
				APIVersion: "agentforge/v1",
				Kind:       model.PluginKindTool,
				Metadata: model.PluginMetadata{
					ID:      "tool.local",
					Name:    "Local Tool",
					Version: "1.0.0",
				},
				Spec: model.PluginSpec{
					Runtime: model.PluginRuntimeMCP,
				},
				Source: model.PluginSource{
					Type: model.PluginSourceLocal,
					Path: "./plugins/tool/manifest.yaml",
					Trust: &model.PluginTrustMetadata{
						Status: model.PluginTrustUntrusted,
					},
				},
			},
			LifecycleState: model.PluginStateInstalled,
			RuntimeHost:    model.PluginHostTSBridge,
		},
	}

	for _, record := range records {
		if err := repo.Save(ctx, record); err != nil {
			t.Fatalf("save plugin record %s: %v", record.Metadata.ID, err)
		}
	}

	h := newPluginHandlerWithRegistry(t, repo, pluginsDir)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/plugins?source=npm&trust=verified", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.List(c); err != nil {
		t.Fatalf("list plugins: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var listed []model.PluginRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("len(listed) = %d, want 1", len(listed))
	}
	if listed[0].Metadata.ID != "review.typescript" {
		t.Fatalf("listed plugin id = %q, want review.typescript", listed[0].Metadata.ID)
	}
}

func TestPluginHandler_InstallSequentialWorkflowAndActivateThroughGoRuntime(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: release-train
  name: Release Train
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/release-train.wasm
  abiVersion: v1
  workflow:
    process: sequential
    roles:
      - id: coder
      - id: reviewer
    steps:
      - id: implement
        role: coder
        action: agent
        next: [review]
      - id: review
        role: reviewer
        action: review
`)

	roleStore := &handlerRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
		},
	}
	goRuntime := &handlerGoRuntime{}
	h := newPluginHandlerWithWorkflowDeps(t, repository.NewPluginRegistryRepository(), pluginsDir, goRuntime, roleStore)
	e := echo.New()

	body, _ := json.Marshal(map[string]string{"path": manifestPath})
	req := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.InstallLocal(c); err != nil {
		t.Fatalf("install workflow: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/release-train/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("release-train")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate workflow: %v", err)
	}
	if activateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", activateRec.Code)
	}
}

func TestPluginHandler_InstallWorkflowRejectsUnknownRoleAndInvalidTransition(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/broken-release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: broken-release-train
  name: Broken Release Train
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/release-train.wasm
  abiVersion: v1
  workflow:
    process: sequential
    roles:
      - id: coder
    steps:
      - id: implement
        role: coder
        action: agent
        next: [missing-review]
      - id: review
        role: reviewer
        action: review
`)

	roleStore := &handlerRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
		},
	}
	h := newPluginHandlerWithWorkflowDeps(t, repository.NewPluginRegistryRepository(), pluginsDir, &handlerGoRuntime{}, roleStore)
	e := echo.New()

	body, _ := json.Marshal(map[string]string{"path": manifestPath})
	req := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.InstallLocal(c); err != nil {
		t.Fatalf("install workflow: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("unknown workflow role reference")) &&
		!bytes.Contains(rec.Body.Bytes(), []byte("unknown workflow step transition")) {
		t.Fatalf("expected workflow validation error, got %s", rec.Body.String())
	}
}

func TestPluginHandler_ActivateUnsupportedWorkflowModeReturnsExplicitError(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/hierarchical-release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: hierarchical-release-train
  name: Hierarchical Release Train
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/release-train.wasm
  abiVersion: v1
  workflow:
    process: hierarchical
    roles:
      - id: coder
    steps:
      - id: implement
        role: coder
        action: agent
`)

	roleStore := &handlerRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
		},
	}
	h := newPluginHandlerWithWorkflowDeps(t, repository.NewPluginRegistryRepository(), pluginsDir, &handlerGoRuntime{}, roleStore)
	e := echo.New()

	body, _ := json.Marshal(map[string]string{"path": manifestPath})
	req := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.InstallLocal(c); err != nil {
		t.Fatalf("install workflow: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	// Hierarchical workflows are now supported — activation should succeed.
	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/hierarchical-release-train/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("hierarchical-release-train")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate workflow: %v", err)
	}
	if activateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 (hierarchical now supported), got %d: %s", activateRec.Code, activateRec.Body.String())
	}
}

func TestPluginHandler_StartWorkflowRunAndQueryStatus(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writePluginManifest(t, pluginsDir, "local/release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: release-train
  name: Release Train
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/release-train.wasm
  abiVersion: v1
  workflow:
    process: sequential
    roles:
      - id: coder
      - id: reviewer
    steps:
      - id: implement
        role: coder
        action: agent
        next: [review]
      - id: review
        role: reviewer
        action: review
`)

	roleStore := &handlerRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
		},
	}
	executor := &handlerWorkflowExecutor{}
	h := newPluginHandlerWithWorkflowRuntime(t, repository.NewPluginRegistryRepository(), pluginsDir, &handlerGoRuntime{}, roleStore, executor)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	installCtx := e.NewContext(installReq, installRec)
	if err := h.InstallLocal(installCtx); err != nil {
		t.Fatalf("install workflow: %v", err)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/release-train/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("release-train")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate workflow: %v", err)
	}

	taskID := uuid.New()
	memberID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"trigger": map[string]any{
			"taskId":   taskID.String(),
			"memberId": memberID.String(),
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/plugins/release-train/workflow-runs", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("release-train")

	if err := h.StartWorkflowRun(c); err != nil {
		t.Fatalf("start workflow run: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var run model.WorkflowPluginRun
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode workflow run: %v", err)
	}
	if run.Status != model.WorkflowRunStatusCompleted {
		t.Fatalf("workflow run status = %s, want completed", run.Status)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("len(executor.calls) = %d, want 2", len(executor.calls))
	}

	listReq := httptest.NewRequest(http.MethodGet, "/plugins/release-train/workflow-runs", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.SetParamNames("id")
	listCtx.SetParamValues("release-train")
	if err := h.ListWorkflowRuns(listCtx); err != nil {
		t.Fatalf("list workflow runs: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}

	var runs []model.WorkflowPluginRun
	if err := json.Unmarshal(listRec.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode workflow run list: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("unexpected workflow runs: %+v", runs)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/plugins/workflow-runs/"+run.ID.String(), nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("runId")
	getCtx.SetParamValues(run.ID.String())
	if err := h.GetWorkflowRun(getCtx); err != nil {
		t.Fatalf("get workflow run: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
}
