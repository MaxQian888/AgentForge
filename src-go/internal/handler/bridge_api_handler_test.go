package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	bridge "github.com/react-go-quick-starter/server/internal/bridge"
)

type bridgeAPIValidator struct {
	validator *validator.Validate
}

func (v *bridgeAPIValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type fakeBridgeAPIClient struct {
	runtimeCatalog *bridge.RuntimeCatalogResponse
	runtimeCalls   int
	runtimeErr     error
	generateReq    *bridge.GenerateRequest
	generateResp   *bridge.GenerateResponse
	generateErr    error
	classifyReq    *bridge.ClassifyIntentRequest
	classifyResp   *bridge.ClassifyIntentResponse
	classifyErr    error
}

func (f *fakeBridgeAPIClient) GetRuntimeCatalog(_ context.Context) (*bridge.RuntimeCatalogResponse, error) {
	f.runtimeCalls++
	if f.runtimeErr != nil {
		return nil, f.runtimeErr
	}
	return f.runtimeCatalog, nil
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
	if client.classifyReq.UserID != "" || client.classifyReq.ProjectID != "" {
		t.Fatalf("expected empty user/project passthrough, got %#v", client.classifyReq)
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

func TestBridgeAPIHandlersWithConcreteBridgeClient(t *testing.T) {
	e := newBridgeAPIHandlerTestEcho()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
}
