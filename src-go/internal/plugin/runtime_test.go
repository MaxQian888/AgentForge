package plugin_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
)

func TestWASMRuntimeManager_ActivateHealthInvokeAndRestart(t *testing.T) {
	modulePath := buildSamplePluginModule(t)
	manager := plugin.NewWASMRuntimeManager()
	record := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata: model.PluginMetadata{
				ID:      "wasm-feishu",
				Name:    "WASM Feishu",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:      model.PluginRuntimeWASM,
				Module:       modulePath,
				ABIVersion:   "v1",
				Capabilities: []string{"health", "send_message"},
				Config: map[string]any{
					"mode": "webhook",
				},
			},
		},
		ResolvedSourcePath: modulePath,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: "v1",
			Compatible: true,
		},
	}

	status, err := manager.ActivatePlugin(context.Background(), record)
	if err != nil {
		t.Fatalf("activate plugin: %v", err)
	}
	if status.LifecycleState != model.PluginStateActive {
		t.Fatalf("expected active state, got %s", status.LifecycleState)
	}

	health, err := manager.CheckPluginHealth(context.Background(), record)
	if err != nil {
		t.Fatalf("check plugin health: %v", err)
	}
	if health.LifecycleState != model.PluginStateActive {
		t.Fatalf("expected active health state, got %s", health.LifecycleState)
	}

	payload, err := manager.Invoke(context.Background(), record, "send_message", map[string]any{
		"chat_id": "chat-1",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("invoke plugin: %v", err)
	}
	if payload["status"] != "sent" {
		t.Fatalf("expected sent status, got %+v", payload)
	}

	restarted, err := manager.RestartPlugin(context.Background(), record)
	if err != nil {
		t.Fatalf("restart plugin: %v", err)
	}
	if restarted.RestartCount != 1 {
		t.Fatalf("expected restart count 1, got %d", restarted.RestartCount)
	}
}

func TestWASMRuntimeManager_RejectsABIMismatch(t *testing.T) {
	modulePath := buildSamplePluginModule(t)
	manager := plugin.NewWASMRuntimeManager()
	record := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata: model.PluginMetadata{
				ID:      "wasm-feishu",
				Name:    "WASM Feishu",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     modulePath,
				ABIVersion: "v2",
			},
		},
		ResolvedSourcePath: modulePath,
	}

	if _, err := manager.ActivatePlugin(context.Background(), record); err == nil {
		t.Fatal("expected ABI mismatch to fail activation")
	}
}

func TestWASMRuntimeManager_RejectsModuleWithoutRequiredExports(t *testing.T) {
	modulePath := buildModuleWithoutExports(t)
	manager := plugin.NewWASMRuntimeManager()
	record := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata: model.PluginMetadata{
				ID:      "missing-exports",
				Name:    "Missing Exports",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     modulePath,
				ABIVersion: "v1",
			},
		},
		ResolvedSourcePath: modulePath,
	}

	if _, err := manager.ActivatePlugin(context.Background(), record); err == nil || !strings.Contains(err.Error(), "missing required export") {
		t.Fatalf("expected missing export error, got %v", err)
	}
}

func TestWASMRuntimeManager_RejectsInvocationOutsideDeclaredCapabilities(t *testing.T) {
	modulePath := buildSamplePluginModule(t)
	manager := plugin.NewWASMRuntimeManager()
	record := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata: model.PluginMetadata{
				ID:      "wasm-feishu",
				Name:    "WASM Feishu",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:      model.PluginRuntimeWASM,
				Module:       modulePath,
				ABIVersion:   "v1",
				Capabilities: []string{"health"},
			},
		},
		ResolvedSourcePath: modulePath,
	}

	if _, err := manager.ActivatePlugin(context.Background(), record); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}
	if _, err := manager.Invoke(context.Background(), record, "send_message", map[string]any{"content": "hello"}); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("expected undeclared capability error, got %v", err)
	}
}

func TestWASMRuntimeManager_InvocationHonorsContextTimeout(t *testing.T) {
	modulePath := buildSlowPluginModule(t)
	manager := plugin.NewWASMRuntimeManager().WithInvocationTimeout(5 * time.Second)
	record := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata: model.PluginMetadata{
				ID:      "wasm-slow",
				Name:    "WASM Slow",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:      model.PluginRuntimeWASM,
				Module:       modulePath,
				ABIVersion:   "v1",
				Capabilities: []string{"hang"},
			},
		},
		ResolvedSourcePath: modulePath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := manager.Invoke(ctx, record, "hang", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("timeout enforcement took too long: %v", time.Since(start))
	}
}

func TestWASMRuntimeManager_EvictsLeastRecentlyUsedCacheEntry(t *testing.T) {
	modulePath := buildSamplePluginModule(t)
	manager := plugin.NewWASMRuntimeManager().WithCacheCapacity(1)

	recordA := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata:   model.PluginMetadata{ID: "wasm-a", Name: "A", Version: "1.0.0"},
			Spec:       model.PluginSpec{Runtime: model.PluginRuntimeWASM, Module: modulePath, ABIVersion: "v1", Capabilities: []string{"health"}},
		},
		ResolvedSourcePath: modulePath,
	}
	recordB := model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindIntegration,
			Metadata:   model.PluginMetadata{ID: "wasm-b", Name: "B", Version: "1.0.0"},
			Spec:       model.PluginSpec{Runtime: model.PluginRuntimeWASM, Module: modulePath, ABIVersion: "v1", Capabilities: []string{"health"}},
		},
		ResolvedSourcePath: modulePath,
	}

	if _, err := manager.CheckPluginHealth(context.Background(), recordA); err != nil {
		t.Fatalf("CheckPluginHealth(A): %v", err)
	}
	if size := manager.CacheSize(); size != 1 {
		t.Fatalf("cache size after A = %d, want 1", size)
	}
	if _, err := manager.CheckPluginHealth(context.Background(), recordB); err != nil {
		t.Fatalf("CheckPluginHealth(B): %v", err)
	}
	if size := manager.CacheSize(); size != 1 {
		t.Fatalf("cache size after B = %d, want 1", size)
	}
	if manager.IsCached("wasm-a") {
		t.Fatal("expected wasm-a to be evicted")
	}
	if !manager.IsCached("wasm-b") {
		t.Fatal("expected wasm-b to remain cached")
	}
}

func buildSamplePluginModule(t *testing.T) string {
	t.Helper()

	outputDir := t.TempDir()
	modulePath := filepath.Join(outputDir, "sample-plugin.wasm")
	buildGoWASMModule(t, filepath.Join("..", ".."), "./cmd/sample-wasm-plugin", modulePath)
	return modulePath
}

func buildModuleWithoutExports(t *testing.T) string {
	t.Helper()

	outputDir := t.TempDir()
	sourceDir := filepath.Join(outputDir, "missing-exports")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir missing-exports source dir: %v", err)
	}
	source := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write missing-exports source: %v", err)
	}

	modulePath := filepath.Join(outputDir, "missing-exports.wasm")
	buildGoWASMModule(t, sourceDir, "main.go", modulePath)
	return modulePath
}

func buildSlowPluginModule(t *testing.T) string {
	t.Helper()

	outputDir := t.TempDir()
	sourceDir := filepath.Join(outputDir, "slow-plugin")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir slow-plugin source dir: %v", err)
	}
	source := `package main

import (
	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

type slowPlugin struct{}

func (slowPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind: "IntegrationPlugin",
		ID: "slow-plugin",
		Name: "Slow Plugin",
		Version: "0.1.0",
		Runtime: "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{{Name: "hang", Description: "spin forever"}},
	}, nil
}

func (slowPlugin) Init(ctx *pluginsdk.Context) error { return nil }
func (slowPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{"status":"ok"}), nil
}
func (slowPlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
	if invocation.Operation == "hang" {
		for {}
	}
	return pluginsdk.Success(map[string]any{"ok": true}), nil
}

var runtime = pluginsdk.NewRuntime(slowPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }

func main() { pluginsdk.Autorun(runtime) }
`
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write slow-plugin source: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	goMod := `module slow-plugin

go 1.25.0

require github.com/agentforge/server v0.0.0

replace github.com/agentforge/server => ` + filepath.ToSlash(repoRoot) + `
`
	if err := os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write slow-plugin go.mod: %v", err)
	}

	modulePath := filepath.Join(outputDir, "slow-plugin.wasm")
	buildGoWASMModule(t, sourceDir, ".", modulePath)
	return modulePath
}

func buildGoWASMModule(t *testing.T, dir, target, output string) {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", output, target)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build wasm module %s: %v\n%s", target, err, string(out))
	}
}

func decodeJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode json payload: %v", err)
	}
	return payload
}
