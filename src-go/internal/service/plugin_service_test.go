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

type fakeGoPluginRuntime struct {
	activated []string
	checked   []string
	restarted []string
	invoked   []string
	operation string
	payload   map[string]any
	result    map[string]any
	status    model.PluginRuntimeStatus
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

func (f *fakeGoPluginRuntime) ActivatePlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	f.activated = append(f.activated, record.Metadata.ID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:           record.Metadata.ID,
			LifecycleState:     model.PluginStateActive,
			Host:               model.PluginHostGoOrchestrator,
			ResolvedSourcePath: record.Spec.Module,
			RuntimeMetadata: &model.PluginRuntimeMetadata{
				ABIVersion: record.Spec.ABIVersion,
				Compatible: true,
			},
		}
	}
	return &status, nil
}

func (f *fakeGoPluginRuntime) CheckPluginHealth(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	f.checked = append(f.checked, record.Metadata.ID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:           record.Metadata.ID,
			LifecycleState:     model.PluginStateActive,
			Host:               model.PluginHostGoOrchestrator,
			ResolvedSourcePath: record.Spec.Module,
			RuntimeMetadata: &model.PluginRuntimeMetadata{
				ABIVersion: record.Spec.ABIVersion,
				Compatible: true,
			},
		}
	}
	return &status, nil
}

func (f *fakeGoPluginRuntime) RestartPlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	f.restarted = append(f.restarted, record.Metadata.ID)
	status := f.status
	if status.PluginID == "" {
		status = model.PluginRuntimeStatus{
			PluginID:           record.Metadata.ID,
			LifecycleState:     model.PluginStateActive,
			Host:               model.PluginHostGoOrchestrator,
			RestartCount:       1,
			ResolvedSourcePath: record.Spec.Module,
			RuntimeMetadata: &model.PluginRuntimeMetadata{
				ABIVersion: record.Spec.ABIVersion,
				Compatible: true,
			},
		}
	}
	return &status, nil
}

func (f *fakeGoPluginRuntime) Invoke(_ context.Context, record model.PluginRecord, operation string, payload map[string]any) (map[string]any, error) {
	f.invoked = append(f.invoked, record.Metadata.ID)
	f.operation = operation
	f.payload = payload
	if f.result == nil {
		f.result = map[string]any{
			"status": "ok",
		}
	}
	return f.result, nil
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
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

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
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), runtime, nil, pluginsDir)

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
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)
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

func TestPluginService_ActivateIntegrationPluginDelegatesToGoRuntime(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/wasm-feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: wasm-feishu
  name: WASM Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)
	goRuntime := &fakeGoPluginRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, goRuntime, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	activated, err := svc.Activate(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("activate integration plugin: %v", err)
	}
	if len(goRuntime.activated) != 1 || goRuntime.activated[0] != record.Metadata.ID {
		t.Fatalf("expected Go runtime activation call, got %+v", goRuntime.activated)
	}
	if activated.ResolvedSourcePath != "./dist/feishu.wasm" {
		t.Fatalf("expected resolved source path to be preserved, got %q", activated.ResolvedSourcePath)
	}
	if activated.RuntimeMetadata == nil || activated.RuntimeMetadata.ABIVersion != "v1" {
		t.Fatalf("expected ABI metadata to be preserved, got %+v", activated.RuntimeMetadata)
	}
}

func TestPluginService_InvokeIntegrationPluginDelegatesToGoRuntime(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/wasm-feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: wasm-feishu
  name: WASM Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
  capabilities: ["send_message"]
`)
	goRuntime := &fakeGoPluginRuntime{
		result: map[string]any{
			"status": "sent",
		},
	}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, goRuntime, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate integration plugin: %v", err)
	}

	result, err := svc.Invoke(ctx, record.Metadata.ID, "send_message", map[string]any{
		"chat_id": "chat-1",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("invoke integration plugin: %v", err)
	}
	if len(goRuntime.invoked) != 1 || goRuntime.invoked[0] != record.Metadata.ID {
		t.Fatalf("expected Go runtime invoke call, got %+v", goRuntime.invoked)
	}
	if goRuntime.operation != "send_message" {
		t.Fatalf("expected send_message operation, got %s", goRuntime.operation)
	}
	if goRuntime.payload["chat_id"] != "chat-1" {
		t.Fatalf("expected payload to reach runtime, got %+v", goRuntime.payload)
	}
	if result["status"] != "sent" {
		t.Fatalf("expected sent result, got %+v", result)
	}
}

func TestPluginService_LegacyGoPluginIntegrationReturnsMigrationError(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/legacy-feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: legacy-feishu
  name: Legacy Feishu
  version: 1.0.0
spec:
  runtime: go-plugin
  binary: ./bin/feishu-adapter
`)
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	if _, err := svc.Activate(ctx, record.Metadata.ID); err == nil {
		t.Fatal("expected legacy go-plugin integration activation to fail with migration guidance")
	}
}
