package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type fakePluginRuntimeClient struct {
	registered []string
	activated  []string
	restarted  []string
	status     model.PluginRuntimeStatus
}

func (f *fakePluginRuntimeClient) RegisterToolPlugin(_ context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	f.registered = append(f.registered, manifest.Metadata.ID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:       manifest.Metadata.ID,
			LifecycleState: model.PluginStateInstalled,
			Host:           model.PluginHostTSBridge,
		}
	}
	return &status, nil
}

func (f *fakePluginRuntimeClient) ActivateToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	f.activated = append(f.activated, pluginID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:       pluginID,
			LifecycleState: model.PluginStateActive,
			Host:           model.PluginHostTSBridge,
		}
	}
	return &status, nil
}

func (f *fakePluginRuntimeClient) CheckToolPluginHealth(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:       pluginID,
			LifecycleState: model.PluginStateActive,
			Host:           model.PluginHostTSBridge,
		}
	}
	return &status, nil
}

func (f *fakePluginRuntimeClient) RestartToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	f.restarted = append(f.restarted, pluginID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:       pluginID,
			LifecycleState: model.PluginStateActive,
			Host:           model.PluginHostTSBridge,
			RestartCount:   1,
		}
	}
	return &status, nil
}

func writeManifest(t *testing.T, dir string, relativePath string, content string) string {
	t.Helper()
	path := filepath.Join(dir, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestPluginService_DiscoversBuiltInsAndFiltersRecords(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/web-search/manifest.yaml", `
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
  args: ["tool.js"]
`)
	writeManifest(t, pluginsDir, "integrations/feishu/manifest.yaml", `
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

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, pluginsDir)

	records, err := svc.DiscoverBuiltIns(ctx)
	if err != nil {
		t.Fatalf("discover built-ins: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 built-ins, got %d", len(records))
	}

	filtered, err := svc.List(ctx, service.PluginListFilter{Kind: model.PluginKindTool})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Metadata.ID != "web-search" {
		t.Fatalf("expected only tool plugin, got %+v", filtered)
	}
}

func TestPluginService_DisabledToolCannotActivateUntilReenabled(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/tool.yaml", `
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
	runtime := &fakePluginRuntimeClient{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), runtime, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}
	if _, err := svc.Disable(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("disable plugin: %v", err)
	}

	if _, err := svc.Activate(ctx, record.Metadata.ID); err == nil {
		t.Fatal("expected disabled plugin activation to fail")
	}

	if _, err := svc.Enable(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate enabled plugin: %v", err)
	}
	if len(runtime.activated) != 1 || runtime.activated[0] != record.Metadata.ID {
		t.Fatalf("expected runtime activation call, got %+v", runtime.activated)
	}
}

func TestPluginService_ReportRuntimeStateUpdatesHealthFields(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/feishu.yaml", `
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
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, pluginsDir)
	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	updated, err := svc.ReportRuntimeState(ctx, record.Metadata.ID, model.PluginRuntimeStatus{
		PluginID:       record.Metadata.ID,
		Host:           model.PluginHostGoOrchestrator,
		LifecycleState: model.PluginStateDegraded,
		LastError:      "health check failed",
		RestartCount:   2,
	})
	if err != nil {
		t.Fatalf("report runtime state: %v", err)
	}

	if updated.LifecycleState != model.PluginStateDegraded {
		t.Fatalf("expected degraded state, got %s", updated.LifecycleState)
	}
	if updated.LastError != "health check failed" {
		t.Fatalf("expected last error to be stored, got %q", updated.LastError)
	}
	if updated.RestartCount != 2 {
		t.Fatalf("expected restart count 2, got %d", updated.RestartCount)
	}
}
