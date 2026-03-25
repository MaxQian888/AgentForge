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

type controlPlanePluginRuntimeClient struct {
	refreshed []string
	called    []string
	read      []string
	prompted  []string
}

func (controlPlanePluginRuntimeClient) RegisterToolPlugin(_ context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       manifest.Metadata.ID,
		LifecycleState: model.PluginStateInstalled,
		Host:           model.PluginHostTSBridge,
	}, nil
}

func (controlPlanePluginRuntimeClient) ActivateToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		Host:           model.PluginHostTSBridge,
	}, nil
}

func (controlPlanePluginRuntimeClient) CheckToolPluginHealth(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		Host:           model.PluginHostTSBridge,
	}, nil
}

func (controlPlanePluginRuntimeClient) RestartToolPlugin(_ context.Context, pluginID string) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		Host:           model.PluginHostTSBridge,
		RestartCount:   1,
	}, nil
}

type controlPlaneGoRuntime struct {
	invoked   []string
	operation string
	payload   map[string]any
	result    map[string]any
}

func (r *controlPlaneGoRuntime) ActivatePlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		LifecycleState:     model.PluginStateActive,
		Host:               model.PluginHostGoOrchestrator,
		ResolvedSourcePath: record.Spec.Module,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
	}, nil
}

func (r *controlPlaneGoRuntime) CheckPluginHealth(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		LifecycleState:     model.PluginStateActive,
		Host:               model.PluginHostGoOrchestrator,
		ResolvedSourcePath: record.Spec.Module,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
	}, nil
}

func (r *controlPlaneGoRuntime) RestartPlugin(_ context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		LifecycleState:     model.PluginStateActive,
		Host:               model.PluginHostGoOrchestrator,
		ResolvedSourcePath: record.Spec.Module,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: record.Spec.ABIVersion,
			Compatible: true,
		},
		RestartCount: 1,
	}, nil
}

func (r *controlPlaneGoRuntime) Invoke(_ context.Context, record model.PluginRecord, operation string, payload map[string]any) (map[string]any, error) {
	r.invoked = append(r.invoked, record.Metadata.ID)
	r.operation = operation
	r.payload = payload
	if r.result == nil {
		r.result = map[string]any{"status": "ok"}
	}
	return r.result, nil
}

type controlPlaneEventBroadcaster struct {
	events []model.PluginEventRecord
}

func (b *controlPlaneEventBroadcaster) BroadcastPluginEvent(event *model.PluginEventRecord) {
	if event == nil {
		return
	}
	b.events = append(b.events, *event)
}

func writeControlPlaneServiceManifest(t *testing.T, dir string, relativePath string, content string) string {
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

func TestPluginServiceControlPlane_PersistsInstanceAndAuditEvent(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneServiceManifest(t, pluginsDir, "local/wasm-feishu.yaml", `
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

	instanceRepo := repository.NewPluginInstanceRepository()
	eventRepo := repository.NewPluginEventRepository()
	broadcaster := &controlPlaneEventBroadcaster{}
	goRuntime := &controlPlaneGoRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &controlPlanePluginRuntimeClient{}, goRuntime, pluginsDir).
		WithInstanceStore(instanceRepo).
		WithEventStore(eventRepo).
		WithBroadcaster(broadcaster)

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	activated, err := svc.Activate(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("activate integration plugin: %v", err)
	}
	if activated.CurrentInstance == nil || activated.CurrentInstance.LifecycleState != model.PluginStateActive {
		t.Fatalf("expected active current instance, got %+v", activated.CurrentInstance)
	}

	snapshot, err := instanceRepo.GetCurrentByPluginID(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("get current instance: %v", err)
	}
	if snapshot.ResolvedSourcePath != "./dist/feishu.wasm" {
		t.Fatalf("resolved source path = %q, want ./dist/feishu.wasm", snapshot.ResolvedSourcePath)
	}

	events, err := eventRepo.ListByPluginID(ctx, record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected plugin events to be recorded")
	}
	if len(broadcaster.events) == 0 {
		t.Fatal("expected activation event to be broadcast")
	}
}

func (r *controlPlanePluginRuntimeClient) RefreshToolPluginMCPSurface(_ context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	r.refreshed = append(r.refreshed, pluginID)
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
				{Name: "search", Description: "Search repository"},
			},
		},
	}, nil
}

func (r *controlPlanePluginRuntimeClient) InvokeToolPluginMCPTool(_ context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	r.called = append(r.called, pluginID+":"+toolName)
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionCallTool),
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: "found 3 files"}},
			IsError: false,
			StructuredContent: map[string]any{
				"count": 3,
				"args":  args,
			},
		},
	}, nil
}

func (r *controlPlanePluginRuntimeClient) ReadToolPluginMCPResource(_ context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	r.read = append(r.read, pluginID+":"+uri)
	return &model.PluginMCPResourceReadResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionReadResource),
		Result: model.MCPResourceReadResult{
			Contents: []model.MCPResourceContent{{URI: uri, MIMEType: "text/markdown", Text: "# README"}},
		},
	}, nil
}

func (r *controlPlanePluginRuntimeClient) GetToolPluginMCPPrompt(_ context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	r.prompted = append(r.prompted, pluginID+":"+name)
	return &model.PluginMCPPromptResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionGetPrompt),
		Result: model.MCPPromptGetResult{
			Description: "Prompt preview",
			Messages: []model.MCPPromptMessage{{
				Role:    "user",
				Content: model.MCPPromptMessageContent{Type: "text", Text: args["topic"]},
			}},
		},
	}, nil
}

func TestPluginServiceControlPlane_RejectsUnsupportedPermissionsAndCapabilities(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneServiceManifest(t, pluginsDir, "local/network-feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: network-feishu
  name: Network Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
  capabilities: ["send_message"]
permissions:
  network:
    required: true
    domains: ["open.feishu.cn"]
`)

	goRuntime := &controlPlaneGoRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &controlPlanePluginRuntimeClient{}, goRuntime, pluginsDir).
		WithPolicy(service.PluginPolicy{AllowNetwork: false, AllowFilesystem: true})

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	if _, err := svc.Activate(ctx, record.Metadata.ID); err == nil {
		t.Fatal("expected activation to fail when required network permission is blocked")
	}

	svc = service.NewPluginService(repository.NewPluginRegistryRepository(), &controlPlanePluginRuntimeClient{}, goRuntime, pluginsDir)
	record, err = svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register local path second pass: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate integration plugin: %v", err)
	}
	if _, err := svc.Invoke(ctx, record.Metadata.ID, "unknown_operation", map[string]any{}); err == nil {
		t.Fatal("expected undeclared capability invocation to fail")
	}
}

func TestPluginServiceControlPlane_ListMarketplaceAndDirectoryInstall(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	builtinPath := writeControlPlaneServiceManifest(t, pluginsDir, "integrations/feishu/manifest.yaml", `
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

	dirPath := filepath.Join(pluginsDir, "local", "dir-plugin")
	writeControlPlaneServiceManifest(t, pluginsDir, "local/dir-plugin/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: dir-plugin
  name: Directory Plugin
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &controlPlanePluginRuntimeClient{}, &controlPlaneGoRuntime{}, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, dirPath)
	if err != nil {
		t.Fatalf("register local directory: %v", err)
	}
	if record.Metadata.ID != "dir-plugin" {
		t.Fatalf("plugin id = %q, want dir-plugin", record.Metadata.ID)
	}

	if _, err := svc.RegisterLocalPath(ctx, builtinPath); err != nil {
		t.Fatalf("register builtin manifest path: %v", err)
	}

	catalog, err := svc.ListMarketplace(ctx)
	if err != nil {
		t.Fatalf("list marketplace: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("expected non-empty catalog")
	}

	var foundBuiltin bool
	for _, item := range catalog {
		if item.ID != "feishu" {
			continue
		}
		foundBuiltin = true
		if !item.Installed {
			t.Fatal("expected installed manifest-backed entry to be marked installed")
		}
		if item.InstallURL == "" {
			t.Fatal("expected manifest-backed install target")
		}
	}
	if !foundBuiltin {
		t.Fatal("expected feishu catalog entry to be present")
	}
}

func TestPluginServiceControlPlane_RefreshMCPPersistsSummaryAndAuditEvent(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneServiceManifest(t, pluginsDir, "local/repo-search.yaml", `
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

	runtimeClient := &controlPlanePluginRuntimeClient{}
	eventRepo := repository.NewPluginEventRepository()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), runtimeClient, nil, pluginsDir).
		WithEventStore(eventRepo)

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register tool plugin: %v", err)
	}
	if _, err := svc.Enable(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("enable tool plugin: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate tool plugin: %v", err)
	}

	refreshed, err := svc.RefreshMCP(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("refresh MCP surface: %v", err)
	}

	if len(runtimeClient.refreshed) != 1 || runtimeClient.refreshed[0] != record.Metadata.ID {
		t.Fatalf("expected runtime refresh call, got %+v", runtimeClient.refreshed)
	}
	if refreshed.RuntimeMetadata == nil || refreshed.RuntimeMetadata.MCP == nil || refreshed.RuntimeMetadata.MCP.ToolCount != 2 {
		t.Fatalf("expected MCP runtime metadata on record, got %+v", refreshed.RuntimeMetadata)
	}

	events, err := eventRepo.ListByPluginID(ctx, record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected MCP refresh event")
	}
	if events[0].EventType != model.PluginEventMCPDiscovery {
		t.Fatalf("expected latest event to be MCP discovery, got %s", events[0].EventType)
	}
}

func TestPluginServiceControlPlane_CallToolUpdatesLatestInteractionSummary(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneServiceManifest(t, pluginsDir, "local/repo-search.yaml", `
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

	runtimeClient := &controlPlanePluginRuntimeClient{}
	eventRepo := repository.NewPluginEventRepository()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), runtimeClient, nil, pluginsDir).
		WithEventStore(eventRepo)

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register tool plugin: %v", err)
	}
	if _, err := svc.Enable(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("enable tool plugin: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate tool plugin: %v", err)
	}

	result, err := svc.CallMCPTool(ctx, record.Metadata.ID, "search", map[string]any{"query": "bridge"})
	if err != nil {
		t.Fatalf("call MCP tool: %v", err)
	}

	if len(runtimeClient.called) != 1 || runtimeClient.called[0] != "repo-search:search" {
		t.Fatalf("expected runtime tool call, got %+v", runtimeClient.called)
	}
	if result.Result.IsError {
		t.Fatalf("expected successful tool call, got %+v", result)
	}

	updated, err := svc.List(ctx, service.PluginListFilter{Kind: model.PluginKindTool})
	if err != nil {
		t.Fatalf("list updated records: %v", err)
	}
	if len(updated) != 1 || updated[0].RuntimeMetadata == nil || updated[0].RuntimeMetadata.MCP == nil || updated[0].RuntimeMetadata.MCP.LatestInteraction == nil {
		t.Fatalf("expected MCP latest interaction summary, got %+v", updated)
	}
	if updated[0].RuntimeMetadata.MCP.LatestInteraction.Operation != model.MCPInteractionCallTool {
		t.Fatalf("unexpected latest interaction: %+v", updated[0].RuntimeMetadata.MCP.LatestInteraction)
	}

	events, err := eventRepo.ListByPluginID(ctx, record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if len(events) == 0 || events[0].EventType != model.PluginEventMCPInteraction {
		t.Fatalf("expected MCP interaction event, got %+v", events)
	}
}

func TestPluginServiceControlPlane_RuntimeStateSyncReconcilesMCPSummary(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeControlPlaneServiceManifest(t, pluginsDir, "local/repo-search.yaml", `
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

	eventRepo := repository.NewPluginEventRepository()
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &controlPlanePluginRuntimeClient{}, nil, pluginsDir).
		WithEventStore(eventRepo)

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register tool plugin: %v", err)
	}

	updated, err := svc.ReportRuntimeState(ctx, record.Metadata.ID, model.PluginRuntimeStatus{
		PluginID:       record.Metadata.ID,
		Host:           model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			MCP: &model.PluginMCPRuntimeMetadata{
				Transport:     "stdio",
				ToolCount:     2,
				ResourceCount: 1,
				PromptCount:   1,
				LatestInteraction: &model.MCPInteractionSummary{
					Operation: model.MCPInteractionReadResource,
					Status:    model.MCPInteractionSucceeded,
					Target:    "file://README.md",
					Summary:   "file://README.md",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("report runtime state: %v", err)
	}

	if updated.RuntimeMetadata == nil || updated.RuntimeMetadata.MCP == nil || updated.RuntimeMetadata.MCP.LatestInteraction == nil {
		t.Fatalf("expected MCP summary in runtime metadata, got %+v", updated.RuntimeMetadata)
	}
	if updated.RuntimeMetadata.MCP.LatestInteraction.Operation != model.MCPInteractionReadResource {
		t.Fatalf("unexpected latest interaction: %+v", updated.RuntimeMetadata.MCP.LatestInteraction)
	}

	events, err := eventRepo.ListByPluginID(ctx, record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if len(events) == 0 || events[0].EventType != model.PluginEventRuntimeSync {
		t.Fatalf("expected runtime sync event tail, got %+v", events)
	}
	if len(events) < 2 || events[1].EventType != model.PluginEventMCPInteraction {
		t.Fatalf("expected MCP interaction audit event before runtime sync, got %+v", events)
	}
}
