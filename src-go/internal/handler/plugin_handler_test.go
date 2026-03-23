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
	"github.com/react-go-quick-starter/server/internal/service"
)

type handlerRuntimeClient struct{}

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
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), handlerRuntimeClient{}, pluginsDir)
	return handler.NewPluginHandler(svc)
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
  runtime: go-plugin
  binary: ./feishu-adapter
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
