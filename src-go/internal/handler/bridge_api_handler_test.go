package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	bridge "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type bridgeAPIValidator struct {
	validator *validator.Validate
}

type bridgeHealthStatusReaderStub struct {
	snapshot service.BridgeHealthSnapshot
}

func (s bridgeHealthStatusReaderStub) Snapshot() service.BridgeHealthSnapshot {
	return s.snapshot
}

func (v *bridgeAPIValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type fakeBridgeAPIClient struct {
	runtimeCatalog *bridge.RuntimeCatalogResponse
	runtimeCalls   int
	runtimeErr     error
	poolResp       *bridge.PoolSummaryResponse
	poolErr        error
	generateReq    *bridge.GenerateRequest
	generateResp   *bridge.GenerateResponse
	generateErr    error
	classifyReq    *bridge.ClassifyIntentRequest
	classifyResp   *bridge.ClassifyIntentResponse
	classifyErr    error
	decomposeReq   *bridge.DecomposeRequest
	decomposeResp  *bridge.DecomposeResponse
	decomposeErr   error
	toolsResp      *bridge.ToolsListResponse
	toolsErr       error
	installReq     *model.PluginManifest
	installResp    *model.PluginRecord
	installErr     error
	uninstallID    string
	uninstallResp  *model.PluginRecord
	uninstallErr   error
	restartID      string
	restartResp    *model.PluginRecord
	restartErr     error
	shellReq       *bridge.ShellRequest
	shellResp      *bridge.ShellResponse
	shellErr       error
	thinkingReq    *bridge.ThinkingBudgetRequest
	thinkingErr    error
	mcpStatusTask  string
	mcpStatusResp  []map[string]any
	mcpStatusErr   error
	authProvider   string
	authStartBody  map[string]any
	authStartResp  map[string]any
	authStartErr   error
	authRequestID  string
	authDoneBody   map[string]any
	authDoneResp   map[string]any
	authDoneErr    error
}

func (f *fakeBridgeAPIClient) GetRuntimeCatalog(_ context.Context) (*bridge.RuntimeCatalogResponse, error) {
	f.runtimeCalls++
	if f.runtimeErr != nil {
		return nil, f.runtimeErr
	}
	return f.runtimeCatalog, nil
}

func (f *fakeBridgeAPIClient) GetPool(_ context.Context) (*bridge.PoolSummaryResponse, error) {
	if f.poolErr != nil {
		return nil, f.poolErr
	}
	return f.poolResp, nil
}

func (f *fakeBridgeAPIClient) Generate(_ context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error) {
	f.generateReq = &req
	if f.generateErr != nil {
		return nil, f.generateErr
	}
	return f.generateResp, nil
}

func (f *fakeBridgeAPIClient) ClassifyIntent(_ context.Context, req bridge.ClassifyIntentRequest) (*bridge.ClassifyIntentResponse, error) {
	f.classifyReq = &req
	if f.classifyErr != nil {
		return nil, f.classifyErr
	}
	return f.classifyResp, nil
}

func (f *fakeBridgeAPIClient) DecomposeTask(_ context.Context, req bridge.DecomposeRequest) (*bridge.DecomposeResponse, error) {
	f.decomposeReq = &req
	if f.decomposeErr != nil {
		return nil, f.decomposeErr
	}
	return f.decomposeResp, nil
}

func (f *fakeBridgeAPIClient) ListTools(_ context.Context) (*bridge.ToolsListResponse, error) {
	if f.toolsErr != nil {
		return nil, f.toolsErr
	}
	return f.toolsResp, nil
}

func (f *fakeBridgeAPIClient) InstallTool(_ context.Context, manifest model.PluginManifest) (*model.PluginRecord, error) {
	f.installReq = &manifest
	if f.installErr != nil {
		return nil, f.installErr
	}
	return f.installResp, nil
}

func (f *fakeBridgeAPIClient) UninstallTool(_ context.Context, pluginID string) (*model.PluginRecord, error) {
	f.uninstallID = pluginID
	if f.uninstallErr != nil {
		return nil, f.uninstallErr
	}
	return f.uninstallResp, nil
}

func (f *fakeBridgeAPIClient) RestartTool(_ context.Context, pluginID string) (*model.PluginRecord, error) {
	f.restartID = pluginID
	if f.restartErr != nil {
		return nil, f.restartErr
	}
	return f.restartResp, nil
}

func (f *fakeBridgeAPIClient) Fork(_ context.Context, req bridge.ForkRequest) (*bridge.ForkResponse, error) {
	return &bridge.ForkResponse{NewTaskID: req.TaskID}, nil
}

func (f *fakeBridgeAPIClient) Rollback(_ context.Context, _ bridge.RollbackRequest) error {
	return nil
}

func (f *fakeBridgeAPIClient) Revert(_ context.Context, _ bridge.RevertRequest) error {
	return nil
}

func (f *fakeBridgeAPIClient) Unrevert(_ context.Context, _ bridge.UnrevertRequest) error {
	return nil
}

func (f *fakeBridgeAPIClient) GetDiff(_ context.Context, _ string) (*bridge.DiffResponse, error) {
	return &bridge.DiffResponse{}, nil
}

func (f *fakeBridgeAPIClient) GetMessages(_ context.Context, _ string) (*bridge.MessagesResponse, error) {
	return &bridge.MessagesResponse{}, nil
}

func (f *fakeBridgeAPIClient) ExecuteCommand(_ context.Context, _ bridge.CommandRequest) error {
	return nil
}

func (f *fakeBridgeAPIClient) ExecuteShell(_ context.Context, req bridge.ShellRequest) (*bridge.ShellResponse, error) {
	f.shellReq = &req
	if f.shellErr != nil {
		return nil, f.shellErr
	}
	return f.shellResp, nil
}

func (f *fakeBridgeAPIClient) Interrupt(_ context.Context, _ string) error {
	return nil
}

func (f *fakeBridgeAPIClient) SwitchModel(_ context.Context, _ bridge.ModelSwitchRequest) error {
	return nil
}

func (f *fakeBridgeAPIClient) SetThinkingBudget(_ context.Context, req bridge.ThinkingBudgetRequest) error {
	f.thinkingReq = &req
	return f.thinkingErr
}

func (f *fakeBridgeAPIClient) GetMCPStatus(_ context.Context, taskID string) ([]map[string]any, error) {
	f.mcpStatusTask = taskID
	if f.mcpStatusErr != nil {
		return nil, f.mcpStatusErr
	}
	return f.mcpStatusResp, nil
}

func (f *fakeBridgeAPIClient) PermissionResponse(_ context.Context, _ string, _ bridge.PermissionResponsePayload) error {
	return nil
}

func (f *fakeBridgeAPIClient) StartOpenCodeProviderAuth(_ context.Context, provider string, payload map[string]any) (map[string]any, error) {
	f.authProvider = provider
	f.authStartBody = payload
	if f.authStartErr != nil {
		return nil, f.authStartErr
	}
	return f.authStartResp, nil
}

func (f *fakeBridgeAPIClient) CompleteOpenCodeProviderAuth(_ context.Context, requestID string, payload map[string]any) (map[string]any, error) {
	f.authRequestID = requestID
	f.authDoneBody = payload
	if f.authDoneErr != nil {
		return nil, f.authDoneErr
	}
	return f.authDoneResp, nil
}

func (f *fakeBridgeAPIClient) GetActive(_ context.Context) ([]bridge.StatusResponse, error) {
	return []bridge.StatusResponse{}, nil
}

func (f *fakeBridgeAPIClient) ListPlugins(_ context.Context) (*bridge.PluginListResponse, error) {
	return &bridge.PluginListResponse{}, nil
}

func (f *fakeBridgeAPIClient) EnablePlugin(_ context.Context, _ string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{}, nil
}

func (f *fakeBridgeAPIClient) DisablePlugin(_ context.Context, _ string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{}, nil
}

func newBridgeAPIHandlerTestEcho() *echo.Echo {
	e := echo.New()
	e.Validator = &bridgeAPIValidator{validator: validator.New()}
	return e
}

func TestBridgeRuntimeCatalogHandler_CachesResponsesForTTL(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	now := time.Now().UTC()
	client := &fakeBridgeAPIClient{
		runtimeCatalog: &bridge.RuntimeCatalogResponse{
			DefaultRuntime: "claude_code",
			Runtimes: []bridge.RuntimeCatalogEntryDTO{
				{Key: "claude_code", Label: "Claude Code", DefaultProvider: "anthropic", DefaultModel: "claude-sonnet-4-5", Available: true},
			},
		},
	}
	handler := newBridgeRuntimeCatalogHandlerWithConfig(client, 60*time.Second, func() time.Time { return now })

	req := httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec := httptest.NewRecorder()
	e.NewContext(req, rec)
	if err := handler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", rec.Code)
	}

	now = now.Add(30 * time.Second)
	req2 := httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec2 := httptest.NewRecorder()
	if err := handler.Get(e.NewContext(req2, rec2)); err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", rec2.Code)
	}
	if client.runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", client.runtimeCalls)
	}

	now = now.Add(31 * time.Second)
	req3 := httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec3 := httptest.NewRecorder()
	if err := handler.Get(e.NewContext(req3, rec3)); err != nil {
		t.Fatalf("third Get() error = %v", err)
	}
	if client.runtimeCalls != 2 {
		t.Fatalf("runtime calls after ttl = %d, want 2", client.runtimeCalls)
	}
}

func TestBridgeRuntimeCatalogHandler_ServiceUnavailableAndBadGateway(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()

	req := httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec := httptest.NewRecorder()
	if err := newBridgeRuntimeCatalogHandlerWithConfig(nil, time.Second, time.Now).Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get(nil client) error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil client status = %d, want 503", rec.Code)
	}

	handler := newBridgeRuntimeCatalogHandlerWithConfig(&fakeBridgeAPIClient{runtimeErr: errors.New("bridge down")}, time.Second, time.Now)
	req = httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec = httptest.NewRecorder()
	if err := handler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get(error client) error = %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("error client status = %d, want 502", rec.Code)
	}

	if NewBridgeRuntimeCatalogHandler((*bridge.Client)(nil)) == nil {
		t.Fatal("NewBridgeRuntimeCatalogHandler() returned nil")
	}
}

func TestBridgeRuntimeCatalogHandler_DefaultConfigFallbacks(t *testing.T) {
	handler := newBridgeRuntimeCatalogHandlerWithConfig(&fakeBridgeAPIClient{}, 0, nil)
	if handler.ttl != 60*time.Second {
		t.Fatalf("handler.ttl = %v, want 60s", handler.ttl)
	}
	if handler.now == nil {
		t.Fatal("handler.now should be initialized")
	}
}

func TestBridgeAIHandler_GenerateProxiesRequest(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		generateResp: &bridge.GenerateResponse{
			Text: "summary",
			Usage: bridge.GenerateUsage{
				InputTokens:  12,
				OutputTokens: 8,
			},
		},
	}
	handler := NewBridgeAIHandler(client)

	req := httptest.NewRequest(http.MethodPost, "/ai/generate", strings.NewReader(`{"prompt":"summarize","provider":"openai","model":"gpt-5"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if err := handler.Generate(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if client.generateReq == nil || client.generateReq.Prompt != "summarize" || client.generateReq.Provider != "openai" || client.generateReq.Model != "gpt-5" {
		t.Fatalf("unexpected proxied request: %#v", client.generateReq)
	}
	var payload bridge.GenerateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Text != "summary" {
		t.Fatalf("text = %q, want summary", payload.Text)
	}
}

func TestBridgeAIHandler_ClassifyIntentProxiesTextPayload(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		classifyResp: &bridge.ClassifyIntentResponse{
			Intent:     "task_assign",
			Command:    "/task assign",
			Args:       "task-1 alice",
			Confidence: 0.9,
			Reply:      "ok",
		},
	}
	handler := NewBridgeAIHandler(client)

	req := httptest.NewRequest(http.MethodPost, "/ai/classify-intent", strings.NewReader(`{"text":"assign this task","candidates":["task_assign","chat"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if err := handler.ClassifyIntent(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ClassifyIntent() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if client.classifyReq == nil || client.classifyReq.Text != "assign this task" {
		t.Fatalf("unexpected proxied classify request: %#v", client.classifyReq)
	}
	if !reflect.DeepEqual(client.classifyReq.Candidates, []string{"task_assign", "chat"}) {
		t.Fatalf("expected candidates passthrough, got %#v", client.classifyReq)
	}
	if client.classifyReq.UserID != "" || client.classifyReq.ProjectID != "" || client.classifyReq.Context != nil {
		t.Fatalf("expected empty user/project/context passthrough, got %#v", client.classifyReq)
	}
}

func TestBridgeAIHandler_ErrorBranches(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()

	nilHandler := NewBridgeAIHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/ai/generate", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if err := nilHandler.Generate(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Generate(nil client) error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil Generate status = %d, want 503", rec.Code)
	}

	client := &fakeBridgeAPIClient{generateErr: errors.New("bridge generate failed")}
	handler := NewBridgeAIHandler(client)
	req = httptest.NewRequest(http.MethodPost, "/ai/generate", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := handler.Generate(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Generate(error) error = %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("Generate(error) status = %d, want 502", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/ai/generate", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := handler.Generate(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Generate(bad json) error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Generate(bad json) status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/ai/classify-intent", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := handler.ClassifyIntent(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ClassifyIntent(validation) error = %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ClassifyIntent(validation) status = %d, want 422", rec.Code)
	}

	client.classifyErr = errors.New("bridge classify failed")
	req = httptest.NewRequest(http.MethodPost, "/ai/classify-intent", strings.NewReader(`{"text":"assign this","candidates":["task_assign"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := handler.ClassifyIntent(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ClassifyIntent(error) error = %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("ClassifyIntent(error) status = %d, want 502", rec.Code)
	}
}

func TestBridgePoolHandler_GetProxiesSummary(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		poolResp: &bridge.PoolSummaryResponse{
			Active:        2,
			Max:           5,
			WarmTotal:     1,
			WarmAvailable: 1,
		},
	}
	handler := NewBridgePoolHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/bridge/pool", nil)
	rec := httptest.NewRecorder()

	if err := handler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload bridge.PoolSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Active != 2 || payload.Max != 5 {
		t.Fatalf("unexpected pool payload: %#v", payload)
	}
}

func TestBridgeAIHandler_DecomposeProxiesContextPayload(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		decomposeResp: &bridge.DecomposeResponse{
			Summary: "Split the work",
			Subtasks: []bridge.DecomposeSubtask{
				{
					Title:         "Add route",
					Description:   "Expose proxy API",
					Priority:      "high",
					ExecutionMode: "agent",
				},
			},
		},
	}
	handler := NewBridgeAIHandler(client)

	req := httptest.NewRequest(http.MethodPost, "/ai/decompose", strings.NewReader(`{"task_id":"task-123","title":"Bridge task","description":"Expose tools and pool proxies","priority":"high","provider":"openai","model":"gpt-5","context":{"relevantFiles":["src-go/internal/server/routes.go"],"waveMode":true}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if err := handler.Decompose(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if client.decomposeReq == nil || client.decomposeReq.TaskID != "task-123" || client.decomposeReq.Provider != "openai" || client.decomposeReq.Model != "gpt-5" {
		t.Fatalf("unexpected proxied decompose request: %#v", client.decomposeReq)
	}
	contextPayload, ok := client.decomposeReq.Context.(map[string]any)
	if !ok || contextPayload["waveMode"] != true {
		t.Fatalf("expected context passthrough, got %#v", client.decomposeReq.Context)
	}
}

func TestBridgeToolsHandler_ListUninstallAndRestart(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		toolsResp: &bridge.ToolsListResponse{
			Tools: []bridge.ToolDefinition{{
				PluginID: "web-search",
				Name:     "search",
			}},
		},
		uninstallResp: &model.PluginRecord{
			PluginManifest: sampleBridgeToolManifest("web-search"),
			LifecycleState: model.PluginStateDisabled,
		},
		restartResp: &model.PluginRecord{
			PluginManifest: sampleBridgeToolManifest("web-search"),
			LifecycleState: model.PluginStateActive,
			RestartCount:   1,
		},
	}
	handler := NewBridgeToolsHandler(client)

	listReq := httptest.NewRequest(http.MethodGet, "/bridge/tools", nil)
	listRec := httptest.NewRecorder()
	if err := handler.List(e.NewContext(listReq, listRec)); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List() status = %d, want 200", listRec.Code)
	}

	uninstallReq := httptest.NewRequest(http.MethodPost, "/bridge/tools/uninstall", strings.NewReader(`{"plugin_id":"web-search"}`))
	uninstallReq.Header.Set("Content-Type", "application/json")
	uninstallRec := httptest.NewRecorder()
	if err := handler.Uninstall(e.NewContext(uninstallReq, uninstallRec)); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if uninstallRec.Code != http.StatusOK {
		t.Fatalf("Uninstall() status = %d, want 200", uninstallRec.Code)
	}
	if client.uninstallID != "web-search" {
		t.Fatalf("expected uninstall plugin id, got %q", client.uninstallID)
	}

	restartReq := httptest.NewRequest(http.MethodPost, "/bridge/tools/web-search/restart", nil)
	restartRec := httptest.NewRecorder()
	restartCtx := e.NewContext(restartReq, restartRec)
	restartCtx.SetPath("/bridge/tools/:id/restart")
	restartCtx.SetParamNames("id")
	restartCtx.SetParamValues("web-search")
	if err := handler.Restart(restartCtx); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	if restartRec.Code != http.StatusOK {
		t.Fatalf("Restart() status = %d, want 200", restartRec.Code)
	}
	if client.restartID != "web-search" {
		t.Fatalf("expected restart plugin id, got %q", client.restartID)
	}
}

func TestBridgeToolsHandler_InstallFetchesManifestFromAllowedURL(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	manifestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: web-search
  name: Web Search
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["index.js"]
permissions: {}
source:
  type: local
`))
	}))
	defer manifestServer.Close()

	parsedURL, err := url.Parse(manifestServer.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	client := &fakeBridgeAPIClient{
		installResp: &model.PluginRecord{
			PluginManifest: sampleBridgeToolManifest("web-search"),
			LifecycleState: model.PluginStateActive,
		},
	}
	handler := NewBridgeToolsHandler(client, parsedURL.Hostname())

	req := httptest.NewRequest(http.MethodPost, "/bridge/tools/install", strings.NewReader(`{"manifest_url":"`+manifestServer.URL+`/manifest.yaml"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if err := handler.Install(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Install() status = %d, want 200", rec.Code)
	}
	if client.installReq == nil || client.installReq.Metadata.ID != "web-search" {
		t.Fatalf("expected parsed manifest to be forwarded, got %#v", client.installReq)
	}
}

func TestBridgeToolsHandler_InstallRejectsDisallowedManifestURL(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{}
	handler := NewBridgeToolsHandler(client, "trusted.example.com")

	req := httptest.NewRequest(http.MethodPost, "/bridge/tools/install", strings.NewReader(`{"manifest_url":"https://untrusted.example.com/manifest.yaml"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if err := handler.Install(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("Install() status = %d, want 403", rec.Code)
	}
	if client.installReq != nil {
		t.Fatalf("expected install not to reach bridge client, got %#v", client.installReq)
	}
}

func sampleBridgeToolManifest(id string) model.PluginManifest {
	return model.PluginManifest{
		APIVersion: "agentforge/v1",
		Kind:       model.PluginKindTool,
		Metadata: model.PluginMetadata{
			ID:      id,
			Name:    "Web Search",
			Version: "1.0.0",
		},
		Spec: model.PluginSpec{
			Runtime:   model.PluginRuntimeMCP,
			Transport: "stdio",
			Command:   "node",
		},
		Permissions: model.PluginPermissions{},
		Source: model.PluginSource{
			Type: model.PluginSourceLocal,
		},
	}
}

func TestBridgeAPIHandlersWithConcreteBridgeClient(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bridge/pool":
			_ = json.NewEncoder(w).Encode(bridge.PoolSummaryResponse{Active: 1, Max: 3})
		case "/bridge/runtimes":
			_ = json.NewEncoder(w).Encode(bridge.RuntimeCatalogResponse{
				DefaultRuntime: "codex",
				Runtimes: []bridge.RuntimeCatalogEntryDTO{{
					Key:             "codex",
					Label:           "Codex",
					DefaultProvider: "openai",
					DefaultModel:    "gpt-5-codex",
					Available:       true,
				}},
			})
		case "/bridge/tools":
			_ = json.NewEncoder(w).Encode(bridge.ToolsListResponse{
				Tools: []bridge.ToolDefinition{{
					PluginID: "web-search",
					Name:     "search",
				}},
			})
		case "/bridge/tools/uninstall":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"apiVersion":      "agentforge/v1",
				"kind":            "ToolPlugin",
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"spec":            map[string]any{"runtime": "mcp", "transport": "stdio", "command": "node"},
				"permissions":     map[string]any{},
				"source":          map[string]any{"type": "local"},
				"lifecycle_state": "disabled",
			})
		case "/bridge/tools/web-search/restart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"apiVersion":      "agentforge/v1",
				"kind":            "ToolPlugin",
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"spec":            map[string]any{"runtime": "mcp", "transport": "stdio", "command": "node"},
				"permissions":     map[string]any{},
				"source":          map[string]any{"type": "local"},
				"lifecycle_state": "active",
				"restart_count":   1,
			})
		case "/bridge/decompose":
			_ = json.NewEncoder(w).Encode(bridge.DecomposeResponse{
				Summary: "ok",
				Subtasks: []bridge.DecomposeSubtask{{
					Title:         "Add proxy",
					Description:   "Expose route",
					Priority:      "high",
					ExecutionMode: "agent",
				}},
			})
		case "/bridge/generate":
			_ = json.NewEncoder(w).Encode(bridge.GenerateResponse{Text: "ok"})
		case "/bridge/classify-intent":
			_ = json.NewEncoder(w).Encode(bridge.ClassifyIntentResponse{Intent: "task_assign", Command: "/task assign"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := bridge.NewClient(server.URL)

	runtimeHandler := NewBridgeRuntimeCatalogHandler(client)
	req := httptest.NewRequest(http.MethodGet, "/bridge/runtimes", nil)
	rec := httptest.NewRecorder()
	if err := runtimeHandler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("runtime handler Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime handler status = %d, want 200", rec.Code)
	}

	poolHandler := NewBridgePoolHandler(client)
	req = httptest.NewRequest(http.MethodGet, "/bridge/pool", nil)
	rec = httptest.NewRecorder()
	if err := poolHandler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("pool handler Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("pool handler status = %d, want 200", rec.Code)
	}

	toolsHandler := NewBridgeToolsHandler(client)
	req = httptest.NewRequest(http.MethodGet, "/bridge/tools", nil)
	rec = httptest.NewRecorder()
	if err := toolsHandler.List(e.NewContext(req, rec)); err != nil {
		t.Fatalf("tools handler List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("tools handler status = %d, want 200", rec.Code)
	}

	aiHandler := NewBridgeAIHandler(client)
	req = httptest.NewRequest(http.MethodPost, "/ai/generate", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := aiHandler.Generate(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Generate(concrete client) error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Generate(concrete client) status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/ai/classify-intent", strings.NewReader(`{"text":"assign this task"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := aiHandler.ClassifyIntent(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ClassifyIntent(concrete client) error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ClassifyIntent(concrete client) status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/ai/decompose", strings.NewReader(`{"task_id":"task-123","title":"Bridge task","description":"Expose route","priority":"high","context":{"waveMode":true}}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := aiHandler.Decompose(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Decompose(concrete client) error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Decompose(concrete client) status = %d, want 200", rec.Code)
	}
}

func TestBridgeConversationHandler_ProxiesAdvancedControlRoutes(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	client := &fakeBridgeAPIClient{
		shellResp: &bridge.ShellResponse{
			Success: true,
			Output:  "lint ok",
		},
		mcpStatusResp: []map[string]any{
			{"name": "github", "healthy": true},
		},
		authStartResp: map[string]any{
			"request_id": "provider-auth-1",
			"provider":   "anthropic",
		},
		authDoneResp: map[string]any{
			"connected": true,
			"provider":  "anthropic",
		},
	}
	handler := NewBridgeConversationHandler(client)

	shellReq := httptest.NewRequest(http.MethodPost, "/bridge/shell", strings.NewReader(`{"task_id":"task-1","command":"pnpm lint","agent":"reviewer"}`))
	shellReq.Header.Set("Content-Type", "application/json")
	shellRec := httptest.NewRecorder()
	if err := handler.ExecuteShell(e.NewContext(shellReq, shellRec)); err != nil {
		t.Fatalf("ExecuteShell() error = %v", err)
	}
	if shellRec.Code != http.StatusOK {
		t.Fatalf("ExecuteShell() status = %d, want 200", shellRec.Code)
	}
	if client.shellReq == nil || client.shellReq.TaskID != "task-1" || client.shellReq.Agent != "reviewer" {
		t.Fatalf("unexpected shell request = %#v", client.shellReq)
	}

	thinkingReq := httptest.NewRequest(http.MethodPost, "/bridge/thinking", strings.NewReader(`{"task_id":"task-1","max_thinking_tokens":4096}`))
	thinkingReq.Header.Set("Content-Type", "application/json")
	thinkingRec := httptest.NewRecorder()
	if err := handler.SetThinkingBudget(e.NewContext(thinkingReq, thinkingRec)); err != nil {
		t.Fatalf("SetThinkingBudget() error = %v", err)
	}
	if thinkingRec.Code != http.StatusOK {
		t.Fatalf("SetThinkingBudget() status = %d, want 200", thinkingRec.Code)
	}
	if client.thinkingReq == nil || client.thinkingReq.MaxThinkingTokens == nil || *client.thinkingReq.MaxThinkingTokens != 4096 {
		t.Fatalf("unexpected thinking request = %#v", client.thinkingReq)
	}

	nullThinkingReq := httptest.NewRequest(http.MethodPost, "/bridge/thinking", strings.NewReader(`{"task_id":"task-1","max_thinking_tokens":null}`))
	nullThinkingReq.Header.Set("Content-Type", "application/json")
	nullThinkingRec := httptest.NewRecorder()
	if err := handler.SetThinkingBudget(e.NewContext(nullThinkingReq, nullThinkingRec)); err != nil {
		t.Fatalf("SetThinkingBudget() null error = %v", err)
	}
	if nullThinkingRec.Code != http.StatusOK {
		t.Fatalf("SetThinkingBudget() null status = %d, want 200", nullThinkingRec.Code)
	}
	if client.thinkingReq == nil || client.thinkingReq.MaxThinkingTokens != nil {
		t.Fatalf("unexpected null thinking request = %#v", client.thinkingReq)
	}

	mcpReq := httptest.NewRequest(http.MethodGet, "/bridge/mcp-status/task-1", nil)
	mcpRec := httptest.NewRecorder()
	mcpCtx := e.NewContext(mcpReq, mcpRec)
	mcpCtx.SetPath("/bridge/mcp-status/:task_id")
	mcpCtx.SetParamNames("task_id")
	mcpCtx.SetParamValues("task-1")
	if err := handler.GetMCPStatus(mcpCtx); err != nil {
		t.Fatalf("GetMCPStatus() error = %v", err)
	}
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("GetMCPStatus() status = %d, want 200", mcpRec.Code)
	}
	if client.mcpStatusTask != "task-1" {
		t.Fatalf("mcp status task = %q, want task-1", client.mcpStatusTask)
	}

	authStartReq := httptest.NewRequest(http.MethodPost, "/bridge/opencode/provider-auth/anthropic/start", strings.NewReader(`{"redirect_uri":"http://127.0.0.1:7777/callback"}`))
	authStartReq.Header.Set("Content-Type", "application/json")
	authStartRec := httptest.NewRecorder()
	authStartCtx := e.NewContext(authStartReq, authStartRec)
	authStartCtx.SetPath("/bridge/opencode/provider-auth/:provider/start")
	authStartCtx.SetParamNames("provider")
	authStartCtx.SetParamValues("anthropic")
	if err := handler.StartOpenCodeProviderAuth(authStartCtx); err != nil {
		t.Fatalf("StartOpenCodeProviderAuth() error = %v", err)
	}
	if authStartRec.Code != http.StatusOK {
		t.Fatalf("StartOpenCodeProviderAuth() status = %d, want 200", authStartRec.Code)
	}
	if client.authProvider != "anthropic" || client.authStartBody["redirect_uri"] != "http://127.0.0.1:7777/callback" {
		t.Fatalf("unexpected provider auth start payload = %#v %#v", client.authProvider, client.authStartBody)
	}

	authDoneReq := httptest.NewRequest(http.MethodPost, "/bridge/opencode/provider-auth/provider-auth-1/complete", strings.NewReader(`{"code":"oauth-code-1","state":"oauth-state-1"}`))
	authDoneReq.Header.Set("Content-Type", "application/json")
	authDoneRec := httptest.NewRecorder()
	authDoneCtx := e.NewContext(authDoneReq, authDoneRec)
	authDoneCtx.SetPath("/bridge/opencode/provider-auth/:request_id/complete")
	authDoneCtx.SetParamNames("request_id")
	authDoneCtx.SetParamValues("provider-auth-1")
	if err := handler.CompleteOpenCodeProviderAuth(authDoneCtx); err != nil {
		t.Fatalf("CompleteOpenCodeProviderAuth() error = %v", err)
	}
	if authDoneRec.Code != http.StatusOK {
		t.Fatalf("CompleteOpenCodeProviderAuth() status = %d, want 200", authDoneRec.Code)
	}
	if client.authRequestID != "provider-auth-1" || client.authDoneBody["code"] != "oauth-code-1" {
		t.Fatalf("unexpected provider auth completion payload = %#v %#v", client.authRequestID, client.authDoneBody)
	}
}

func TestBridgePoolAndHealthHandlers_HandleConcurrentRequests(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	e.GET("/bridge/pool", NewBridgePoolHandler(&fakeBridgeAPIClient{
		poolResp: &bridge.PoolSummaryResponse{
			Active:        2,
			Max:           5,
			WarmTotal:     1,
			WarmAvailable: 1,
		},
	}).Get)
	e.GET("/bridge/health", NewBridgeHealthHandler(bridgeHealthStatusReaderStub{
		snapshot: service.BridgeHealthSnapshot{
			Status:    service.BridgeStatusReady,
			LastCheck: time.Unix(1_700_000_000, 0).UTC(),
			Pool: service.BridgeHealthPool{
				Active:    2,
				Available: 3,
				Warm:      1,
			},
		},
	}).Get)
	srv := httptest.NewServer(e)
	defer srv.Close()

	client := srv.Client()
	paths := []string{"/bridge/pool", "/bridge/health"}
	var wg sync.WaitGroup
	errCh := make(chan error, 64)

	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 10; iteration++ {
				for _, path := range paths {
					resp, err := client.Get(srv.URL + path)
					if err != nil {
						errCh <- err
						return
					}
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						errCh <- errors.New("unexpected status code")
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent bridge handler request failed: %v", err)
		}
	}
}
