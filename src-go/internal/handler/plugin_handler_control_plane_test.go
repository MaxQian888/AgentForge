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
