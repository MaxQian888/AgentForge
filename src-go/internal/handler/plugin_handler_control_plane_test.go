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

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
)

type controlPlaneRuntimeClient struct{}

func (controlPlaneRuntimeClient) RegisterToolPlugin(_ context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       manifest.Metadata.ID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateInstalled,
	}, nil
}

func (controlPlaneRuntimeClient) ActivateToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (controlPlaneRuntimeClient) CheckToolPluginHealth(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (controlPlaneRuntimeClient) RestartToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
		RestartCount:   1,
	}, nil
}

type controlPlaneGoRuntime struct{}

func (controlPlaneGoRuntime) ActivatePlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
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

func (controlPlaneGoRuntime) CheckPluginHealth(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
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

func (controlPlaneGoRuntime) RestartPlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
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

func (controlPlaneGoRuntime) Invoke(_ context.Context, _ model.PluginRecord, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"status": "ok"}, nil
}

func writeControlPlaneManifest(t *testing.T, dir string, relative string, content string) string {
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

func newControlPlanePluginHandler(pluginsDir string, goRuntime service.GoPluginRuntime) *handler.PluginHandler {
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), controlPlaneRuntimeClient{}, goRuntime, pluginsDir)
	return handler.NewPluginHandler(svc)
}

func TestPluginHandlerControlPlane_MarketplaceReturnsManifestBackedCatalog(t *testing.T) {
	pluginsDir := t.TempDir()
	writeControlPlaneManifest(t, pluginsDir, "integrations/feishu/manifest.yaml", `
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

	h := newControlPlanePluginHandler(pluginsDir, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/plugins/marketplace", nil)
	rec := httptest.NewRecorder()

	if err := h.Marketplace(e.NewContext(req, rec)); err != nil {
		t.Fatalf("marketplace: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var plugins []model.MarketplacePluginDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &plugins); err != nil {
		t.Fatalf("decode marketplace response: %v", err)
	}
	if len(plugins) != 1 || plugins[0].ID != "feishu" {
		t.Fatalf("unexpected marketplace payload: %+v", plugins)
	}
	if plugins[0].InstallURL == "" {
		t.Fatal("expected install url to come from manifest path")
	}
}

func (controlPlaneRuntimeClient) RefreshToolPluginMCPSurface(_ context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	return &model.PluginMCPRefreshResult{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostTSBridge,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			MCP: &model.PluginMCPRuntimeMetadata{
				Transport:     "stdio",
				ToolCount:     2,
				ResourceCount: 1,
				PromptCount:   1,
			},
		},
		Snapshot: model.PluginMCPCapabilitySnapshot{
			Transport:     "stdio",
			ToolCount:     2,
			ResourceCount: 1,
			PromptCount:   1,
			Tools: []model.MCPCapabilityTool{
				{Name: "search"},
			},
		},
	}, nil
}

func (controlPlaneRuntimeClient) InvokeToolPluginMCPTool(_ context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionCallTool),
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: "found 3 files"}},
		},
	}, nil
}

func (controlPlaneRuntimeClient) ReadToolPluginMCPResource(_ context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	return &model.PluginMCPResourceReadResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionReadResource),
		Result: model.MCPResourceReadResult{
			Contents: []model.MCPResourceContent{{URI: uri, MIMEType: "text/markdown", Text: "# README"}},
		},
	}, nil
}

func (controlPlaneRuntimeClient) GetToolPluginMCPPrompt(_ context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	return &model.PluginMCPPromptResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionGetPrompt),
		Result: model.MCPPromptGetResult{
			Description: "Prompt preview",
			Messages: []model.MCPPromptMessage{{
				Role:    "user",
				Content: model.MCPPromptMessageContent{Type: "text", Text: name},
			}},
		},
	}, nil
}

func TestPluginHandlerControlPlane_RuntimeStateSync(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneManifest(t, pluginsDir, "local/feishu.yaml", `
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

	h := newControlPlanePluginHandler(pluginsDir, nil)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallLocal(e.NewContext(installReq, installRec)); err != nil {
		t.Fatalf("install local: %v", err)
	}

	updateBody, _ := json.Marshal(model.PluginRuntimeStatus{
		PluginID:       "feishu",
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateDegraded,
		LastError:      "health check failed",
		RestartCount:   2,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/internal/plugins/runtime-state", bytes.NewReader(updateBody))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()

	if err := h.SyncRuntimeState(e.NewContext(updateReq, updateRec)); err != nil {
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
	if record.RestartCount != 2 {
		t.Fatalf("expected restart count 2, got %d", record.RestartCount)
	}
}

func TestPluginHandlerControlPlane_ListEventsReturnsAuditTrail(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneManifest(t, pluginsDir, "local/feishu.yaml", `
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

	h := newControlPlanePluginHandler(pluginsDir, controlPlaneGoRuntime{})
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallLocal(e.NewContext(installReq, installRec)); err != nil {
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
}

func TestPluginHandlerControlPlane_MCPRefreshAndCallRoutes(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneManifest(t, pluginsDir, "local/repo-search.yaml", `
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

	h := newControlPlanePluginHandler(pluginsDir, nil)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallLocal(e.NewContext(installReq, installRec)); err != nil {
		t.Fatalf("install local: %v", err)
	}

	enableReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/enable", nil)
	enableRec := httptest.NewRecorder()
	enableCtx := e.NewContext(enableReq, enableRec)
	enableCtx.SetParamNames("id")
	enableCtx.SetParamValues("repo-search")
	if err := h.Enable(enableCtx); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("repo-search")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/mcp/refresh", nil)
	refreshRec := httptest.NewRecorder()
	refreshCtx := e.NewContext(refreshReq, refreshRec)
	refreshCtx.SetParamNames("id")
	refreshCtx.SetParamValues("repo-search")
	if err := h.RefreshMCP(refreshCtx); err != nil {
		t.Fatalf("refresh MCP: %v", err)
	}
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", refreshRec.Code)
	}

	var refreshResp model.PluginMCPRefreshResult
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshResp); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshResp.Snapshot.ToolCount != 2 || len(refreshResp.Snapshot.Tools) != 1 {
		t.Fatalf("unexpected refresh payload: %+v", refreshResp)
	}

	callBody, _ := json.Marshal(map[string]any{
		"tool_name": "search",
		"arguments": map[string]any{"query": "bridge"},
	})
	callReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/mcp/tools/call", bytes.NewReader(callBody))
	callReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	callRec := httptest.NewRecorder()
	callCtx := e.NewContext(callReq, callRec)
	callCtx.SetParamNames("id")
	callCtx.SetParamValues("repo-search")
	if err := h.CallMCPTool(callCtx); err != nil {
		t.Fatalf("call MCP tool: %v", err)
	}
	if callRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", callRec.Code)
	}

	var callResp model.PluginMCPToolCallResult
	if err := json.Unmarshal(callRec.Body.Bytes(), &callResp); err != nil {
		t.Fatalf("decode tool call response: %v", err)
	}
	if callResp.PluginID != "repo-search" || callResp.Operation != string(model.MCPInteractionCallTool) {
		t.Fatalf("unexpected tool call payload: %+v", callResp)
	}
}

func TestPluginHandlerControlPlane_MCPCallValidation(t *testing.T) {
	h := newControlPlanePluginHandler(t.TempDir(), nil)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/mcp/tools/call", bytes.NewReader([]byte(`{}`)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("repo-search")

	if err := h.CallMCPTool(c); err != nil {
		t.Fatalf("call MCP tool: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPluginHandlerControlPlane_MCPResourceAndPromptRoutes(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneManifest(t, pluginsDir, "local/repo-search.yaml", `
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

	h := newControlPlanePluginHandler(pluginsDir, nil)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallLocal(e.NewContext(installReq, installRec)); err != nil {
		t.Fatalf("install local: %v", err)
	}

	enableReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/enable", nil)
	enableRec := httptest.NewRecorder()
	enableCtx := e.NewContext(enableReq, enableRec)
	enableCtx.SetParamNames("id")
	enableCtx.SetParamValues("repo-search")
	if err := h.Enable(enableCtx); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/activate", nil)
	activateRec := httptest.NewRecorder()
	activateCtx := e.NewContext(activateReq, activateRec)
	activateCtx.SetParamNames("id")
	activateCtx.SetParamValues("repo-search")
	if err := h.Activate(activateCtx); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}

	resourceBody, _ := json.Marshal(map[string]any{"uri": "file://README.md"})
	resourceReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/mcp/resources/read", bytes.NewReader(resourceBody))
	resourceReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	resourceRec := httptest.NewRecorder()
	resourceCtx := e.NewContext(resourceReq, resourceRec)
	resourceCtx.SetParamNames("id")
	resourceCtx.SetParamValues("repo-search")
	if err := h.ReadMCPResource(resourceCtx); err != nil {
		t.Fatalf("read MCP resource: %v", err)
	}
	if resourceRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resourceRec.Code)
	}

	promptBody, _ := json.Marshal(map[string]any{
		"name": "summarize",
		"arguments": map[string]string{
			"topic": "repo-search",
		},
	})
	promptReq := httptest.NewRequest(http.MethodPost, "/plugins/repo-search/mcp/prompts/get", bytes.NewReader(promptBody))
	promptReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	promptRec := httptest.NewRecorder()
	promptCtx := e.NewContext(promptReq, promptRec)
	promptCtx.SetParamNames("id")
	promptCtx.SetParamValues("repo-search")
	if err := h.GetMCPPrompt(promptCtx); err != nil {
		t.Fatalf("get MCP prompt: %v", err)
	}
	if promptRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", promptRec.Code)
	}
}

func TestPluginHandlerControlPlane_CatalogSearchAndInstallRoutes(t *testing.T) {
	pluginsDir := t.TempDir()
	writeControlPlaneManifest(t, pluginsDir, "catalog/review-typescript/manifest.yaml", `
apiVersion: agentforge/v1
kind: ReviewPlugin
metadata:
  id: review-typescript
  name: TypeScript Review
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["review.js"]
  review:
    entrypoint: review:run
    triggers:
      events: ["pull_request.updated"]
    output:
      format: findings/v1
source:
  type: catalog
  catalog: internal
  entry: review-typescript
  digest: sha256:review-typescript
  signature: sigstore-bundle
  trust:
    status: verified
    approvalState: approved
`)

	h := newControlPlanePluginHandler(pluginsDir, nil)
	e := echo.New()

	searchReq := httptest.NewRequest(http.MethodGet, "/plugins/catalog?q=typescript", nil)
	searchRec := httptest.NewRecorder()
	if err := h.SearchCatalog(e.NewContext(searchReq, searchRec)); err != nil {
		t.Fatalf("search catalog: %v", err)
	}
	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", searchRec.Code)
	}

	installBody, _ := json.Marshal(map[string]string{"entry_id": "review-typescript"})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/catalog/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallCatalogEntry(e.NewContext(installReq, installRec)); err != nil {
		t.Fatalf("install catalog entry: %v", err)
	}
	if installRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", installRec.Code)
	}
}

func TestPluginHandlerControlPlane_DeactivateAndUpdateRoutes(t *testing.T) {
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneManifest(t, pluginsDir, "local/release-train-v1.yaml", `
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
    steps:
      - id: implement
        role: coder
        action: agent
`)
	updatedManifestPath := writeControlPlaneManifest(t, pluginsDir, "local/release-train-v2.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: release-train
  name: Release Train
  version: 1.1.0
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
`)

	roleStore := &controlPlaneHandlerRoleStore{
		roles: map[string]struct{}{"coder": {}},
	}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), controlPlaneRuntimeClient{}, controlPlaneGoRuntime{}, pluginsDir).
		WithRoleStore(roleStore)
	h := handler.NewPluginHandler(svc)
	e := echo.New()

	installBody, _ := json.Marshal(map[string]string{"path": manifestPath})
	installReq := httptest.NewRequest(http.MethodPost, "/plugins/install", bytes.NewReader(installBody))
	installReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	installRec := httptest.NewRecorder()
	if err := h.InstallLocal(e.NewContext(installReq, installRec)); err != nil {
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

	deactivateReq := httptest.NewRequest(http.MethodPost, "/plugins/release-train/deactivate", nil)
	deactivateRec := httptest.NewRecorder()
	deactivateCtx := e.NewContext(deactivateReq, deactivateRec)
	deactivateCtx.SetParamNames("id")
	deactivateCtx.SetParamValues("release-train")
	if err := h.Deactivate(deactivateCtx); err != nil {
		t.Fatalf("deactivate workflow: %v", err)
	}
	if deactivateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deactivateRec.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"path": updatedManifestPath,
		"source": map[string]any{
			"type":       "git",
			"repository": "https://github.com/example/release-train.git",
			"ref":        "refs/tags/v1.1.0",
			"digest":     "sha256:release-train-v2",
			"signature":  "sigstore-bundle",
		},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/plugins/release-train/update", bytes.NewReader(updateBody))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues("release-train")
	if err := h.Update(updateCtx); err != nil {
		t.Fatalf("update workflow: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateRec.Code)
	}
}

type controlPlaneHandlerRoleStore struct {
	roles map[string]struct{}
}

func (s *controlPlaneHandlerRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if _, ok := s.roles[id]; !ok {
		return nil, os.ErrNotExist
	}
	return &rolepkg.Manifest{Metadata: model.RoleMetadata{ID: id, Name: id}}, nil
}
