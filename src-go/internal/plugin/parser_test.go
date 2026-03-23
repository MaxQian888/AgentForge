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
