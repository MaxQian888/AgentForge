package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
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

type fakePluginEventBroadcaster struct {
	events []model.PluginEventRecord
}

type fakePluginRoleStore struct {
	roles map[string]*rolepkg.Manifest
}

func (f *fakePluginEventBroadcaster) BroadcastPluginEvent(event *model.PluginEventRecord) {
	if event == nil {
		return
	}
	f.events = append(f.events, *event)
}

func (f *fakePluginRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if f == nil || f.roles == nil {
		return nil, os.ErrNotExist
	}
	role, ok := f.roles[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return role, nil
}

func (f *fakePluginRoleStore) List() ([]*rolepkg.Manifest, error) {
	if f == nil || f.roles == nil {
		return nil, nil
	}
	roles := make([]*rolepkg.Manifest, 0, len(f.roles))
	for _, role := range f.roles {
		roles = append(roles, role)
	}
	return roles, nil
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

func (f *fakePluginRuntimeClient) RefreshToolPluginMCPSurface(_ context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	return &model.PluginMCPRefreshResult{
		PluginID:       pluginID,
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostTSBridge,
		Snapshot: model.PluginMCPCapabilitySnapshot{
			Transport: "stdio",
		},
	}, nil
}

func (f *fakePluginRuntimeClient) InvokeToolPluginMCPTool(_ context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	return &model.PluginMCPToolCallResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionCallTool),
		Result: model.MCPToolCallResult{
			Content: []model.MCPContentBlock{{Type: "text", Text: toolName}},
		},
	}, nil
}

func (f *fakePluginRuntimeClient) ReadToolPluginMCPResource(_ context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	return &model.PluginMCPResourceReadResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionReadResource),
		Result: model.MCPResourceReadResult{
			Contents: []model.MCPResourceContent{{URI: uri}},
		},
	}, nil
}

func (f *fakePluginRuntimeClient) GetToolPluginMCPPrompt(_ context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	return &model.PluginMCPPromptResult{
		PluginID:  pluginID,
		Operation: string(model.MCPInteractionGetPrompt),
		Result: model.MCPPromptGetResult{
			Description: name,
		},
	}, nil
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

func writeBuiltInBundle(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "builtin-bundle.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write built-in bundle: %v", err)
	}
	return path
}

func TestPluginService_DiscoversBuiltInsWithoutInstallingRecords(t *testing.T) {
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
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: web-search
    kind: ToolPlugin
    manifest: tools/web-search/manifest.yaml
    docsRef: docs/part/PLUGIN_SYSTEM_DESIGN.md#七工具插件系统tool-plugin
    verificationProfile: mcp-tool
    availability:
      status: ready
      message: Bundled and ready for install.
  - id: feishu
    kind: IntegrationPlugin
    manifest: integrations/feishu/manifest.yaml
    docsRef: docs/GO_WASM_PLUGIN_RUNTIME.md
    verificationProfile: go-wasm
    availability:
      status: requires_configuration
      message: Requires Feishu application credentials before live activation.
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
	if len(filtered) != 0 {
		t.Fatalf("expected discover to leave installed registry empty, got %+v", filtered)
	}
}

func TestPluginService_DiscoverBuiltInsUsesOfficialBundleMetadata(t *testing.T) {
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
	writeManifest(t, pluginsDir, "reviews/architecture-check/manifest.yaml", `
apiVersion: agentforge/v1
kind: ReviewPlugin
metadata:
  id: architecture-check
  name: Architecture Check
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
      filePatterns: ["src/**/*.ts"]
    output:
      format: findings/v1
`)
	writeManifest(t, pluginsDir, "tools/experimental-helper/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: experimental-helper
  name: Experimental Helper
  version: 0.0.1
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["experimental.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: web-search
    kind: ToolPlugin
    manifest: tools/web-search/manifest.yaml
    docsRef: docs/part/PLUGIN_SYSTEM_DESIGN.md#七工具插件系统tool-plugin
    verificationProfile: mcp-tool
    availability:
      status: ready
      message: Bundled and ready for install.
  - id: architecture-check
    kind: ReviewPlugin
    manifest: reviews/architecture-check/manifest.yaml
    docsRef: docs/part/PLUGIN_SYSTEM_DESIGN.md#十审查插件系统review-plugin
    verificationProfile: mcp-review
    availability:
      status: ready
      message: Runs through the built-in deep review flow.
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	records, err := svc.DiscoverBuiltIns(ctx)
	if err != nil {
		t.Fatalf("discover built-ins: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 official built-ins, got %d", len(records))
	}

	ids := []string{records[0].Metadata.ID, records[1].Metadata.ID}
	if strings.Contains(strings.Join(ids, ","), "experimental-helper") {
		t.Fatalf("expected unlisted built-in manifest to be skipped, got %+v", ids)
	}

	var architecture *model.PluginRecord
	for _, record := range records {
		if record.Metadata.ID == "architecture-check" {
			architecture = record
			break
		}
	}
	if architecture == nil {
		t.Fatalf("expected architecture-check record, got %+v", ids)
	}
	if architecture.BuiltIn == nil {
		t.Fatalf("expected built-in bundle metadata, got %+v", architecture)
	}
	if architecture.BuiltIn.VerificationProfile != "mcp-review" {
		t.Fatalf("verification profile = %q, want mcp-review", architecture.BuiltIn.VerificationProfile)
	}
	if architecture.BuiltIn.AvailabilityStatus != "ready" {
		t.Fatalf("availability status = %q, want ready", architecture.BuiltIn.AvailabilityStatus)
	}
	if architecture.BuiltIn.ReadinessStatus != "ready" {
		t.Fatalf("readiness status = %q, want ready", architecture.BuiltIn.ReadinessStatus)
	}
}

func TestPluginService_DiscoverBuiltInsEvaluatesStructuredReadiness(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/github-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: github-tool
  name: GitHub Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: github-tool
    kind: ToolPlugin
    manifest: tools/github-tool/manifest.yaml
    docsRef: docs/PRD.md#46-工具插件系统tool-plugin
    verificationProfile: mcp-tool
    readiness:
      readyMessage: GitHub tool is ready for install.
      blockedMessage: GitHub tool still needs local setup before activation.
      nextStep: Configure bridge credentials and install the required helper runtime.
      installable: true
      prerequisites:
        - kind: executable
          value: definitely-missing-agentforge-cli
          label: AgentForge CLI helper
      configuration:
        - kind: env
          value: AGENTFORGE_GITHUB_TOKEN
          label: GitHub token
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	records, err := svc.DiscoverBuiltIns(ctx)
	if err != nil {
		t.Fatalf("discover built-ins: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 built-in, got %d", len(records))
	}

	record := records[0]
	if record.BuiltIn == nil {
		t.Fatalf("expected built-in metadata, got %+v", record)
	}
	if record.BuiltIn.ReadinessStatus != "requires_prerequisite" {
		t.Fatalf("readiness status = %q, want requires_prerequisite", record.BuiltIn.ReadinessStatus)
	}
	if record.BuiltIn.NextStep != "Configure bridge credentials and install the required helper runtime." {
		t.Fatalf("next step = %q", record.BuiltIn.NextStep)
	}
	if !record.BuiltIn.Installable {
		t.Fatal("expected built-in to remain installable when only activation readiness is blocked")
	}
	if len(record.BuiltIn.MissingPrerequisites) != 1 || record.BuiltIn.MissingPrerequisites[0] != "AgentForge CLI helper" {
		t.Fatalf("missing prerequisites = %+v", record.BuiltIn.MissingPrerequisites)
	}
	if len(record.BuiltIn.MissingConfiguration) != 0 {
		t.Fatalf("expected prerequisite failure to take precedence, got %+v", record.BuiltIn.MissingConfiguration)
	}
	if len(record.BuiltIn.BlockingReasons) == 0 {
		t.Fatalf("expected blocking reasons, got %+v", record.BuiltIn.BlockingReasons)
	}
}

func TestPluginService_SearchCatalogSeparatesInstallabilityFromReadiness(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/github-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: github-tool
  name: GitHub Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: github-tool
    kind: ToolPlugin
    manifest: tools/github-tool/manifest.yaml
    docsRef: docs/PRD.md#46-工具插件系统tool-plugin
    verificationProfile: mcp-tool
    readiness:
      readyMessage: GitHub tool is ready for install.
      blockedMessage: GitHub tool still needs bridge credentials before activation.
      nextStep: Set AGENTFORGE_GITHUB_TOKEN on the bridge host.
      installable: true
      configuration:
        - kind: env
          value: AGENTFORGE_GITHUB_TOKEN
          label: GitHub token
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	catalog, err := svc.SearchCatalog(ctx, "github")
	if err != nil {
		t.Fatalf("search catalog: %v", err)
	}
	if len(catalog) != 1 {
		t.Fatalf("expected 1 catalog entry, got %d", len(catalog))
	}
	entry := catalog[0]
	if !entry.Installable {
		t.Fatal("expected built-in entry to remain installable")
	}
	if entry.BuiltIn == nil {
		t.Fatalf("expected built-in metadata, got %+v", entry)
	}
	if entry.BuiltIn.ReadinessStatus != "requires_configuration" {
		t.Fatalf("readiness status = %q, want requires_configuration", entry.BuiltIn.ReadinessStatus)
	}
	if len(entry.BuiltIn.MissingConfiguration) != 1 || entry.BuiltIn.MissingConfiguration[0] != "GitHub token" {
		t.Fatalf("missing configuration = %+v", entry.BuiltIn.MissingConfiguration)
	}
}

func TestPluginService_InstalledBuiltInRetainsReadinessMetadata(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/github-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: github-tool
  name: GitHub Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: github-tool
    kind: ToolPlugin
    manifest: tools/github-tool/manifest.yaml
    docsRef: docs/PRD.md#46-工具插件系统tool-plugin
    verificationProfile: mcp-tool
    readiness:
      readyMessage: GitHub tool is ready for install.
      blockedMessage: GitHub tool still needs bridge credentials before activation.
      nextStep: Set AGENTFORGE_GITHUB_TOKEN on the bridge host.
      installable: true
      configuration:
        - kind: env
          value: AGENTFORGE_GITHUB_TOKEN
          label: GitHub token
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	record, err := svc.InstallCatalogEntry(ctx, "github-tool")
	if err != nil {
		t.Fatalf("install catalog entry: %v", err)
	}
	if record.BuiltIn == nil {
		t.Fatalf("expected built-in metadata after install, got %+v", record)
	}
	if record.BuiltIn.ReadinessStatus != "requires_configuration" {
		t.Fatalf("readiness status after install = %q, want requires_configuration", record.BuiltIn.ReadinessStatus)
	}

	hydrated, err := svc.GetByID(ctx, "github-tool")
	if err != nil {
		t.Fatalf("get installed built-in: %v", err)
	}
	if hydrated.BuiltIn == nil {
		t.Fatalf("expected hydrated built-in metadata, got %+v", hydrated)
	}
	if hydrated.BuiltIn.NextStep != "Set AGENTFORGE_GITHUB_TOKEN on the bridge host." {
		t.Fatalf("next step = %q", hydrated.BuiltIn.NextStep)
	}
}

func TestPluginService_DiscoverBuiltInsMarksUnsupportedHostAsBlocked(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/host-specific-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: host-specific-tool
  name: Host Specific Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: host-specific-tool
    kind: ToolPlugin
    manifest: tools/host-specific-tool/manifest.yaml
    docsRef: docs/PRD.md#46-工具插件系统tool-plugin
    verificationProfile: mcp-tool
    readiness:
      readyMessage: Host-specific tool is ready for install.
      blockedMessage: Host-specific tool is not supported on this host.
      nextStep: Use a supported host family for this built-in.
      installable: false
      supportedHosts: ["definitely-unsupported-host"]
`)

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	records, err := svc.DiscoverBuiltIns(ctx)
	if err != nil {
		t.Fatalf("discover built-ins: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 built-in, got %d", len(records))
	}
	record := records[0]
	if record.BuiltIn == nil {
		t.Fatalf("expected built-in metadata, got %+v", record)
	}
	if record.BuiltIn.ReadinessStatus != "unsupported_host" {
		t.Fatalf("readiness status = %q, want unsupported_host", record.BuiltIn.ReadinessStatus)
	}
	if record.BuiltIn.Installable {
		t.Fatal("expected unsupported host built-in to be non-installable")
	}
	if record.BuiltIn.InstallBlockedReason == "" {
		t.Fatal("expected install blocked reason for unsupported host")
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

func TestPluginService_ActivateIntegrationPluginPersistsCurrentInstanceAndAuditEvent(t *testing.T) {
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
	instanceRepo := repository.NewPluginInstanceRepository()
	eventRepo := repository.NewPluginEventRepository()
	broadcaster := &fakePluginEventBroadcaster{}
	goRuntime := &fakeGoPluginRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, goRuntime, pluginsDir).
		WithInstanceStore(instanceRepo).
		WithEventStore(eventRepo).
		WithBroadcaster(broadcaster)

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	activated, err := svc.Activate(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("activate integration plugin: %v", err)
	}
	if activated.CurrentInstance == nil {
		t.Fatal("expected current instance to be attached after activation")
	}
	if activated.CurrentInstance.LifecycleState != model.PluginStateActive {
		t.Fatalf("instance lifecycle state = %s, want active", activated.CurrentInstance.LifecycleState)
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

func TestPluginService_ActivateRejectsUnsupportedRequiredPermissions(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/network-tool.yaml", `
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
permissions:
  network:
    required: true
    domains: ["open.feishu.cn"]
`)
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir).
		WithPolicy(service.PluginPolicy{AllowNetwork: false, AllowFilesystem: true})

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}

	if _, err := svc.Activate(ctx, record.Metadata.ID); err == nil {
		t.Fatal("expected activation to fail when required network permission is blocked")
	}
}

func TestPluginService_ListMarketplaceUsesManifestCatalogAndRegistryState(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	builtinPath := writeManifest(t, pluginsDir, "integrations/feishu/manifest.yaml", `
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
	localPath := writeManifest(t, pluginsDir, "local/repo-search.yaml", `
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
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir)

	if _, err := svc.RegisterLocalPath(ctx, localPath); err != nil {
		t.Fatalf("register local path: %v", err)
	}
	if _, err := svc.RegisterLocalPath(ctx, builtinPath); err != nil {
		t.Fatalf("register builtin-like path: %v", err)
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
		if item.ID == "feishu" {
			foundBuiltin = true
			if !item.Installed {
				t.Fatal("expected installed manifest-backed entry to be marked installed")
			}
			if item.InstallURL == "" {
				t.Fatal("expected manifest-backed install target")
			}
		}
		if item.ID == "role-coder" {
			t.Fatal("did not expect placeholder marketplace entry")
		}
	}
	if !foundBuiltin {
		t.Fatal("expected feishu catalog entry to be present")
	}
}

func TestPluginService_RegisterLocalPathAcceptsPluginDirectory(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	pluginDir := filepath.Join(pluginsDir, "local", "dir-plugin")
	writeManifest(t, pluginsDir, "local/dir-plugin/manifest.yaml", `
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

	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, nil, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, pluginDir)
	if err != nil {
		t.Fatalf("register local directory: %v", err)
	}
	if record.Metadata.ID != "dir-plugin" {
		t.Fatalf("plugin id = %q, want dir-plugin", record.Metadata.ID)
	}
	if record.Source.Path == "" {
		t.Fatal("expected source path to be populated from manifest.yaml")
	}
}

func TestPluginService_RegisterWorkflowValidatesRolesAndTransitions(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/broken-release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: broken-release-train
  name: Broken Release Train
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
        next: [missing-review]
      - id: review
        role: reviewer
        action: review
`)
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir).
		WithRoleStore(&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		})

	if _, err := svc.RegisterLocalPath(ctx, localPath); err == nil {
		t.Fatal("expected invalid workflow registration to fail")
	}
}

func TestPluginService_ActivateSequentialWorkflowDelegatesToGoRuntime(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/release-train.yaml", `
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
      - id: reviewer
    steps:
      - id: implement
        role: coder
        action: agent
        next: [review]
      - id: review
        role: reviewer
        action: review
`)
	goRuntime := &fakeGoPluginRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, goRuntime, pluginsDir).
		WithRoleStore(&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		})

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	activated, err := svc.Activate(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("activate workflow: %v", err)
	}
	if len(goRuntime.activated) != 1 || goRuntime.activated[0] != record.Metadata.ID {
		t.Fatalf("expected Go runtime activation call, got %+v", goRuntime.activated)
	}
	if activated.LifecycleState != model.PluginStateActive {
		t.Fatalf("workflow lifecycle state = %s, want active", activated.LifecycleState)
	}
}

func TestPluginService_ActivateHierarchicalWorkflowReturnsUnsupportedError(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/hierarchical-release-train.yaml", `
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: hierarchical-release-train
  name: Hierarchical Release Train
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/release-train.wasm
  abiVersion: v1
  workflow:
    process: hierarchical
    roles:
      - id: coder
    steps:
      - id: implement
        role: coder
        action: agent
`)
	goRuntime := &fakeGoPluginRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, goRuntime, pluginsDir).
		WithRoleStore(&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		})

	record, err := svc.RegisterLocalPath(ctx, localPath)
	if err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	// Hierarchical workflows are now supported — activation should succeed.
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		if strings.Contains(err.Error(), "unsupported workflow process") {
			t.Fatal("hierarchical workflow should no longer be rejected as unsupported")
		}
		// Other errors (e.g., WASM module not found) are acceptable
	}
}

func TestPluginService_GetByIDHydratesWorkflowRoleDependencies(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	repo := repository.NewPluginRegistryRepository()
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "workflow.release-train",
				Name:    "Release Train",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/release-train.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: model.WorkflowProcessSequential,
					Roles: []model.WorkflowRoleBinding{
						{ID: "coder"},
						{ID: "reviewer"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "implement", Role: "coder", Action: model.WorkflowActionAgent, Next: []string{"review"}},
						{ID: "review", Role: "reviewer", Action: model.WorkflowActionReview},
					},
				},
			},
		},
		LifecycleState: model.PluginStateEnabled,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("save plugin record: %v", err)
	}

	svc := service.NewPluginService(repo, &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir).
		WithRoleStore(&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		})

	hydrated, err := svc.GetByID(ctx, "workflow.release-train")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(hydrated.RoleDependencies) != 2 {
		t.Fatalf("len(RoleDependencies) = %d, want 2", len(hydrated.RoleDependencies))
	}
	if hydrated.RoleDependencies[0].RoleID != "coder" || hydrated.RoleDependencies[0].Status != "resolved" {
		t.Fatalf("first role dependency = %+v, want resolved coder", hydrated.RoleDependencies[0])
	}
	if hydrated.RoleDependencies[1].RoleID != "reviewer" || hydrated.RoleDependencies[1].Status != "missing" || !hydrated.RoleDependencies[1].Blocking {
		t.Fatalf("second role dependency = %+v, want missing blocking reviewer", hydrated.RoleDependencies[1])
	}
}

func TestPluginService_GetByIDHydratesToolPluginRoleConsumers(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	repo := repository.NewPluginRegistryRepository()
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindTool,
			Metadata: model.PluginMetadata{
				ID:      "design-mcp",
				Name:    "Design MCP",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:   model.PluginRuntimeMCP,
				Transport: "stdio",
				Command:   "node",
			},
		},
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostTSBridge,
	}
	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("save plugin record: %v", err)
	}

	svc := service.NewPluginService(repo, &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir).
		WithRoleStore(&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"design-lead": {
					Metadata: model.RoleMetadata{ID: "design-lead", Name: "Design Lead"},
					Capabilities: model.RoleCapabilities{
						ToolConfig: model.RoleToolConfig{
							External: []string{"design-mcp"},
						},
					},
				},
			},
		})

	hydrated, err := svc.GetByID(ctx, "design-mcp")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(hydrated.RoleConsumers) != 1 {
		t.Fatalf("len(RoleConsumers) = %d, want 1", len(hydrated.RoleConsumers))
	}
	if hydrated.RoleConsumers[0].RoleID != "design-lead" || hydrated.RoleConsumers[0].Status != "active" {
		t.Fatalf("role consumer = %+v, want active design-lead consumer", hydrated.RoleConsumers[0])
	}
}

func TestPluginService_EnableRejectsWorkflowPluginWithStaleRoleReference(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	localPath := writeManifest(t, pluginsDir, "local/release-train.yaml", `
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
      - id: reviewer
    steps:
      - id: implement
        role: coder
        action: agent
        next: [review]
      - id: review
        role: reviewer
        action: review
`)
	roleStore := &fakePluginRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
		},
	}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir).
		WithRoleStore(roleStore)
	record, err := svc.Install(ctx, service.PluginInstallRequest{Path: localPath})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	delete(roleStore.roles, "reviewer")

	_, err = svc.Enable(ctx, record.Metadata.ID)
	if err == nil || !strings.Contains(err.Error(), "unknown workflow role reference: reviewer") {
		t.Fatalf("Enable() error = %v, want stale reviewer role failure", err)
	}
}
