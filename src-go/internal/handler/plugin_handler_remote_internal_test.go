package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type internalRemoteRegistryClient struct {
	entries      []service.RemotePluginEntry
	manifestBody string
	fetchErr     error
	downloadErr  error
}

func (c *internalRemoteRegistryClient) FetchCatalog(_ context.Context, _ string) ([]service.RemotePluginEntry, error) {
	if c.fetchErr != nil {
		return nil, c.fetchErr
	}
	return c.entries, nil
}

func (c *internalRemoteRegistryClient) Download(_ context.Context, _, _, _ string) (io.ReadCloser, error) {
	if c.downloadErr != nil {
		return nil, c.downloadErr
	}
	return io.NopCloser(bytes.NewBufferString(c.manifestBody)), nil
}

type internalPluginRuntimeClient struct{}

func (internalPluginRuntimeClient) RegisterToolPlugin(_ context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       manifest.Metadata.ID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateInstalled,
	}, nil
}

func (internalPluginRuntimeClient) ActivateToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (internalPluginRuntimeClient) CheckToolPluginHealth(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (internalPluginRuntimeClient) RestartToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}, nil
}

func (internalPluginRuntimeClient) RefreshToolPluginMCPSurface(_ context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	return &model.PluginMCPRefreshResult{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostTSBridge,
		Snapshot: model.PluginMCPCapabilitySnapshot{
			Transport: "stdio",
		},
	}, nil
}

func (internalPluginRuntimeClient) InvokeToolPluginMCPTool(_ context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionCallTool),
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: toolName}},
		},
	}, nil
}

func (internalPluginRuntimeClient) ReadToolPluginMCPResource(_ context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	return &model.PluginMCPResourceReadResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionReadResource),
		Result: model.MCPResourceReadResult{
			Contents: []model.MCPResourceContent{{URI: uri}},
		},
	}, nil
}

func (internalPluginRuntimeClient) GetToolPluginMCPPrompt(_ context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	return &model.PluginMCPPromptResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionGetPrompt),
		Result: model.MCPPromptGetResult{
			Description: name,
		},
	}, nil
}

func newInternalRemotePluginHandler(t *testing.T, pluginsDir string, client service.RemoteRegistryClient, registryURL string) *PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), internalPluginRuntimeClient{}, nil, pluginsDir)
	svc.SetRemoteRegistry(client, registryURL)
	return NewPluginHandler(svc)
}

func TestInternalRemotePluginHandlerListReturnsUnavailableEnvelope(t *testing.T) {
	h := newInternalRemotePluginHandler(t, t.TempDir(), nil, "")
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/plugins/marketplace/remote", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ListRemotePlugins(c); err != nil {
		t.Fatalf("ListRemotePlugins() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload struct {
		Available bool   `json:"available"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Available {
		t.Fatal("expected available=false when registry is unconfigured")
	}
	if payload.ErrorCode != "remote_registry_unconfigured" {
		t.Fatalf("errorCode = %q, want remote_registry_unconfigured", payload.ErrorCode)
	}
}

func TestInternalRemotePluginHandlerInstallReturnsVerificationFailure(t *testing.T) {
	h := newInternalRemotePluginHandler(t, t.TempDir(), &internalRemoteRegistryClient{
		manifestBody: `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: release-train
  name: Release Train
  version: 1.2.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["plugin.js"]
`,
	}, "https://registry.agentforge.dev")
	e := echo.New()
	body, _ := json.Marshal(map[string]string{"version": "1.2.0"})
	req := httptest.NewRequest(http.MethodPost, "/plugins/marketplace/release-train/install-remote", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("release-train")

	if err := h.InstallRemotePlugin(c); err != nil {
		t.Fatalf("InstallRemotePlugin() error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}

	var payload struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ErrorCode != "remote_registry_verification_failed" {
		t.Fatalf("errorCode = %q, want remote_registry_verification_failed", payload.ErrorCode)
	}
}
