package plugin_test

import (
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/plugin"
)

func TestParse_ValidToolPluginManifest(t *testing.T) {
	data := []byte(`
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
permissions:
  network:
    required: true
    domains: ["example.com"]
`)

	manifest, err := plugin.Parse(data)
	if err != nil {
		t.Fatalf("expected manifest to parse, got error: %v", err)
	}

	if manifest.Kind != model.PluginKindTool {
		t.Fatalf("expected tool kind, got %s", manifest.Kind)
	}

	if manifest.Spec.Runtime != model.PluginRuntimeMCP {
		t.Fatalf("expected MCP runtime, got %s", manifest.Spec.Runtime)
	}
}

func TestParse_InvalidRuntimeForKind(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: feishu
  name: Feishu
  version: 1.0.0
spec:
  runtime: mcp
  command: node
`)

	if _, err := plugin.Parse(data); err == nil {
		t.Fatal("expected invalid kind/runtime combination to fail")
	}
}

func TestParse_ValidWASMIntegrationPluginManifest(t *testing.T) {
	data := []byte(`
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
  capabilities: ["events:inbound", "messages:outbound"]
`)

	manifest, err := plugin.Parse(data)
	if err != nil {
		t.Fatalf("expected wasm manifest to parse, got error: %v", err)
	}

	if manifest.Spec.Runtime != model.PluginRuntimeWASM {
		t.Fatalf("expected WASM runtime, got %s", manifest.Spec.Runtime)
	}
	if manifest.Spec.Module != "./dist/feishu.wasm" {
		t.Fatalf("expected module path to be preserved, got %q", manifest.Spec.Module)
	}
	if manifest.Spec.ABIVersion != "v1" {
		t.Fatalf("expected ABI version to be preserved, got %q", manifest.Spec.ABIVersion)
	}
}

func TestParse_WASMIntegrationPluginRequiresModuleAndABIVersion(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: wasm-feishu
  name: WASM Feishu
  version: 1.0.0
spec:
  runtime: wasm
`)

	if _, err := plugin.Parse(data); err == nil {
		t.Fatal("expected wasm integration manifest without module and abiVersion to fail")
	}
}
