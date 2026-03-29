package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type handlerRemoteRegistryClient struct {
	entries      []service.RemotePluginEntry
	manifestBody string
	fetchErr     error
	downloadErr  error
}

func (f *handlerRemoteRegistryClient) FetchCatalog(_ context.Context, _ string) ([]service.RemotePluginEntry, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.entries, nil
}

func (f *handlerRemoteRegistryClient) Download(_ context.Context, _, _, _ string) (io.ReadCloser, error) {
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return io.NopCloser(bytes.NewBufferString(f.manifestBody)), nil
}

func newPluginHandlerWithRemoteRegistry(
	t *testing.T,
	pluginsDir string,
	client service.RemoteRegistryClient,
	registryURL string,
) *handler.PluginHandler {
	t.Helper()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), handlerRuntimeClient{}, nil, pluginsDir)
	svc.SetRemoteRegistry(client, registryURL)
	return handler.NewPluginHandler(svc)
}

func TestPluginHandler_ListRemotePluginsReturnsUnavailableEnvelopeWhenRegistryMissing(t *testing.T) {
	h := newPluginHandler(t, t.TempDir())
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
		Available bool              `json:"available"`
		ErrorCode string            `json:"errorCode"`
		Error     string            `json:"error"`
		Registry  string            `json:"registry"`
		Entries   []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode remote marketplace payload: %v", err)
	}
	if payload.Available {
		t.Fatal("expected available=false when no remote registry is configured")
	}
	if payload.ErrorCode != "remote_registry_unconfigured" {
		t.Fatalf("errorCode = %q, want remote_registry_unconfigured", payload.ErrorCode)
	}
	if len(payload.Entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(payload.Entries))
	}
}

func TestPluginHandler_ListRemotePluginsNormalizesMarketplaceEntries(t *testing.T) {
	h := newPluginHandlerWithRemoteRegistry(t, t.TempDir(), &handlerRemoteRegistryClient{
		entries: []service.RemotePluginEntry{
			{
				PluginID:    "release-train",
				Name:        "Release Train",
				Version:     "1.2.0",
				Description: "Workflow release automation",
				Author:      "AgentForge",
				Tags:        []string{"workflow", "release"},
			},
		},
	}, "https://registry.agentforge.dev")
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
		Registry  string `json:"registry"`
		Entries   []struct {
			ID          string `json:"id"`
			Registry    string `json:"registry"`
			Installable bool   `json:"installable"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode remote marketplace payload: %v", err)
	}
	if !payload.Available {
		t.Fatal("expected available=true for configured registry response")
	}
	if payload.Registry != "https://registry.agentforge.dev" {
		t.Fatalf("registry = %q, want https://registry.agentforge.dev", payload.Registry)
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(payload.Entries))
	}
	if payload.Entries[0].ID != "release-train" {
		t.Fatalf("entry id = %q, want release-train", payload.Entries[0].ID)
	}
	if payload.Entries[0].Registry != "https://registry.agentforge.dev" {
		t.Fatalf("entry registry = %q, want https://registry.agentforge.dev", payload.Entries[0].Registry)
	}
	if !payload.Entries[0].Installable {
		t.Fatal("expected remote entry to be marked installable")
	}
}

func TestPluginHandler_InstallRemotePluginReturnsClassifiedTrustFailure(t *testing.T) {
	h := newPluginHandlerWithRemoteRegistry(t, t.TempDir(), &handlerRemoteRegistryClient{
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
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode remote install payload: %v", err)
	}
	if payload.ErrorCode != "remote_registry_verification_failed" {
		t.Fatalf("errorCode = %q, want remote_registry_verification_failed", payload.ErrorCode)
	}
}
