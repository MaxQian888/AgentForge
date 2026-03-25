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

func TestParse_ValidWorkflowPluginManifest(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: release-train
  name: Release Train
  version: 1.2.0
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
    triggers:
      - event: manual
source:
  type: git
  repository: https://github.com/example/release-train.git
  ref: refs/tags/v1.2.0
  digest: sha256:workflow
  trust:
    status: verified
    approvalState: approved
  release:
    version: 1.2.0
    channel: stable
`)

	manifest, err := plugin.Parse(data)
	if err != nil {
		t.Fatalf("expected workflow manifest to parse, got error: %v", err)
	}

	if manifest.Spec.Runtime != model.PluginRuntimeWASM {
		t.Fatalf("expected workflow runtime wasm, got %s", manifest.Spec.Runtime)
	}
	if manifest.Spec.Workflow == nil || manifest.Spec.Workflow.Process != "sequential" {
		t.Fatalf("expected sequential workflow spec, got %+v", manifest.Spec.Workflow)
	}
	if manifest.Source.Type != model.PluginSourceGit {
		t.Fatalf("expected git source type, got %s", manifest.Source.Type)
	}
	if manifest.Source.Trust == nil || manifest.Source.Trust.Status != model.PluginTrustVerified {
		t.Fatalf("expected verified trust metadata, got %+v", manifest.Source.Trust)
	}
}

func TestParse_ValidReviewPluginManifest(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: ReviewPlugin
metadata:
  id: review-typescript
  name: TypeScript Review
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["dist/review.js"]
  review:
    entrypoint: review:run
    triggers:
      events: ["pull_request.updated"]
      filePatterns: ["src/**/*.ts"]
    output:
      format: findings/v1
source:
  type: npm
  package: "@agentforge/review-typescript"
  version: 1.0.0
  registry: https://registry.npmjs.org
  digest: sha256:review
  signature: sigstore-bundle
  trust:
    status: verified
    approvalState: approved
`)

	manifest, err := plugin.Parse(data)
	if err != nil {
		t.Fatalf("expected review manifest to parse, got error: %v", err)
	}

	if manifest.Spec.Review == nil || manifest.Spec.Review.Output.Format != "findings/v1" {
		t.Fatalf("expected normalized review output format, got %+v", manifest.Spec.Review)
	}
	if manifest.Source.Type != model.PluginSourceNPM {
		t.Fatalf("expected npm source type, got %s", manifest.Source.Type)
	}
}

func TestParse_ReviewPluginRejectsUnsupportedOutputFormat(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: ReviewPlugin
metadata:
  id: review-comments
  name: Review Comments
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["dist/review.js"]
  review:
    entrypoint: review:run
    triggers:
      events: ["pull_request.updated"]
    output:
      format: github-review-comments
`)

	if _, err := plugin.Parse(data); err == nil {
		t.Fatal("expected unsupported review output format to fail")
	}
}

func TestParse_WorkflowPluginRejectsLegacyGoPluginRuntime(t *testing.T) {
	data := []byte(`
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: legacy-workflow
  name: Legacy Workflow
  version: 0.9.0
spec:
  runtime: go-plugin
  binary: ./bin/workflow
  workflow:
    process: sequential
    roles:
      - id: coder
    steps:
      - id: implement
        role: coder
        action: agent
`)

	if _, err := plugin.Parse(data); err == nil {
		t.Fatal("expected workflow manifest using legacy go-plugin runtime to fail")
	}
}
