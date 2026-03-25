/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");

const { getRepoRoot } = require("./plugin-dev-targets.js");

const DEFAULT_MODULE_PATH = "github.com/react-go-quick-starter/server";

function parseArgs(argv = process.argv.slice(2)) {
  const parsed = {};

  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--type") {
      parsed.type = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--name") {
      parsed.name = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--root") {
      parsed.rootDir = argv[index + 1];
      index += 1;
    }
  }

  return parsed;
}

function scaffoldPlugin(options) {
  const rootDir = path.resolve(options.rootDir || getRepoRoot());
  const type = normalizeType(options.type);
  const name = normalizeName(options.name);
  const displayName = toDisplayName(name);
  const pluginId = name;
  const version = "0.1.0";
  const modulePath = resolveGoModulePath(rootDir);

  switch (type) {
    case "tool":
      return scaffoldToolPlugin({ rootDir, name, displayName, pluginId, version });
    case "review":
      return scaffoldReviewPlugin({ rootDir, name, displayName, pluginId, version });
    case "workflow":
      return scaffoldWorkflowPlugin({ rootDir, name, displayName, pluginId, version, modulePath });
    case "integration":
      return scaffoldIntegrationPlugin({ rootDir, name, displayName, pluginId, version, modulePath });
    default:
      throw new Error(`unsupported plugin type ${type}`);
  }
}

function normalizeType(value) {
  const normalized = String(value || "").trim().toLowerCase();
  if (!normalized) {
    throw new Error("plugin type is required");
  }
  if (!["tool", "review", "workflow", "integration"].includes(normalized)) {
    throw new Error(`unsupported plugin type ${normalized}`);
  }
  return normalized;
}

function normalizeName(value) {
  const normalized = String(value || "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  if (!normalized) {
    throw new Error("plugin name is required");
  }

  return normalized;
}

function toDisplayName(name) {
  return name
    .split("-")
    .filter(Boolean)
    .map((segment) => segment[0].toUpperCase() + segment.slice(1))
    .join(" ");
}

function resolveGoModulePath(rootDir) {
  const goModPath = path.join(rootDir, "src-go", "go.mod");
  if (!fs.existsSync(goModPath)) {
    return DEFAULT_MODULE_PATH;
  }

  const source = fs.readFileSync(goModPath, "utf8");
  const match = source.match(/^module\s+([^\r\n]+)$/m);
  return match?.[1]?.trim() || DEFAULT_MODULE_PATH;
}

function ensureDir(targetPath) {
  fs.mkdirSync(targetPath, { recursive: true });
}

function writeFile(filePath, source) {
  ensureDir(path.dirname(filePath));
  fs.writeFileSync(filePath, source);
}

function toImportPath(fromFile, targetFile) {
  const relativePath = path.relative(path.dirname(fromFile), targetFile).replace(/\\/g, "/");
  return relativePath.startsWith(".") ? relativePath : `./${relativePath}`;
}

function writeJson(filePath, value) {
  writeFile(filePath, `${JSON.stringify(value, null, 2)}\n`);
}

function scaffoldToolPlugin({ rootDir, name, displayName, pluginId, version }) {
  const pluginDir = path.join(rootDir, "plugins", "tools", name);
  const entryPath = path.join(pluginDir, "src", "index.ts");
  const testPath = path.join(pluginDir, "src", "index.test.ts");
  const sdkImport = toImportPath(entryPath, path.join(rootDir, "src-bridge", "src", "plugin-sdk", "index.js"));

  writeFile(
    path.join(pluginDir, "manifest.yaml"),
    [
      "apiVersion: agentforge/v1",
      "kind: ToolPlugin",
      "metadata:",
      `  id: ${pluginId}`,
      `  name: ${displayName}`,
      `  version: ${version}`,
      `  description: Starter MCP tool plugin for ${displayName}.`,
      "spec:",
      "  runtime: mcp",
      "  transport: stdio",
      "  command: bun",
      "  args:",
      "    - run",
      "    - src/index.ts",
      "permissions: {}",
      "source:",
      "  type: local",
      `  path: ./plugins/tools/${name}/manifest.yaml`,
      "",
    ].join("\n"),
  );

  writeJson(path.join(pluginDir, "package.json"), {
    name: `@agentforge/tool-${name}`,
    private: true,
    type: "module",
    scripts: {
      dev: "bun run src/index.ts",
      test: "bun test src/index.test.ts",
      validate: "bun test src/index.test.ts",
    },
  });

  writeFile(
    entryPath,
    `import { z } from "zod";
import {
  connectPluginStdioServer,
  createPluginMcpServer,
  createToolPluginManifest,
  defineToolPlugin,
} from "${sdkImport}";

export const manifest = createToolPluginManifest({
  id: "${pluginId}",
  name: "${displayName}",
  version: "${version}",
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  capabilities: ["echo"],
});

export const plugin = defineToolPlugin({
  manifest,
  tools: [
    {
      name: "echo",
      description: "Echo text back to the caller.",
      inputSchema: z.object({ text: z.string() }),
      execute: async ({ text }) => ({
        content: [{ type: "text", text: \`Echo: \${text}\` }],
      }),
    },
  ],
});

if (import.meta.main) {
  const server = createPluginMcpServer(plugin);
  await connectPluginStdioServer(server);
}
`,
  );

  writeFile(
    testPath,
    `import { describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "${sdkImport}";
import { plugin } from "./index.js";

describe("${displayName} tool plugin", () => {
  test("echoes input through the local plugin harness", async () => {
    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("echo", { text: "hello" });
    const text = result.content.find((item) => item.type === "text");
    expect(text?.text).toBe("Echo: hello");
  });
});
`,
  );

  return { pluginDir };
}

function scaffoldReviewPlugin({ rootDir, name, displayName, pluginId, version }) {
  const pluginDir = path.join(rootDir, "plugins", "reviews", name);
  const entryPath = path.join(pluginDir, "src", "index.ts");
  const testPath = path.join(pluginDir, "src", "index.test.ts");
  const sdkImport = toImportPath(entryPath, path.join(rootDir, "src-bridge", "src", "plugin-sdk", "index.js"));

  writeFile(
    path.join(pluginDir, "manifest.yaml"),
    [
      "apiVersion: agentforge/v1",
      "kind: ReviewPlugin",
      "metadata:",
      `  id: ${pluginId}`,
      `  name: ${displayName}`,
      `  version: ${version}`,
      `  description: Starter review plugin for ${displayName}.`,
      "spec:",
      "  runtime: mcp",
      "  transport: stdio",
      "  command: bun",
      "  args:",
      "    - run",
      "    - src/index.ts",
      "  review:",
      "    entrypoint: review:run",
      "    triggers:",
      "      events:",
      "        - pull_request.updated",
      "      filePatterns:",
      "        - src/**/*.ts",
      "    output:",
      "      format: findings/v1",
      "permissions: {}",
      "source:",
      "  type: local",
      `  path: ./plugins/reviews/${name}/manifest.yaml`,
      "",
    ].join("\n"),
  );

  writeJson(path.join(pluginDir, "package.json"), {
    name: `@agentforge/review-${name}`,
    private: true,
    type: "module",
    scripts: {
      dev: "bun run src/index.ts",
      test: "bun test src/index.test.ts",
      validate: "bun test src/index.test.ts",
    },
  });

  writeFile(
    entryPath,
    `import {
  connectPluginStdioServer,
  createPluginMcpServer,
  createReviewPluginManifest,
  createReviewResult,
  defineReviewPlugin,
} from "${sdkImport}";

export const manifest = createReviewPluginManifest({
  id: "${pluginId}",
  name: "${displayName}",
  version: "${version}",
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  triggers: {
    events: ["pull_request.updated"],
    filePatterns: ["src/**/*.ts"],
  },
});

export const plugin = defineReviewPlugin({
  manifest,
  executeReview: async ({ review_id }) =>
    createReviewResult({
      pluginId: "${pluginId}",
      summary: \`Review \${String(review_id)} produced one finding.\`,
      findings: [
        {
          category: "starter",
          severity: "low",
          file: "src/index.ts",
          line: 1,
          message: "Replace the starter review rule with your team-specific checks.",
        },
      ],
    }),
});

if (import.meta.main) {
  const server = createPluginMcpServer(plugin);
  await connectPluginStdioServer(server);
}
`,
  );

  writeFile(
    testPath,
    `import { describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "${sdkImport}";
import { plugin } from "./index.js";

describe("${displayName} review plugin", () => {
  test("emits normalized findings through the local plugin harness", async () => {
    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:run", { review_id: "review-123" });
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        format: "findings/v1",
      }),
    );
  });
});
`,
  );

  return { pluginDir };
}

function scaffoldWorkflowPlugin({ rootDir, name, displayName, pluginId, version, modulePath }) {
  const pluginDir = path.join(rootDir, "plugins", "workflows", name);
  const goSourceDir = path.join(rootDir, "src-go", "cmd", name);
  const buildScript = toImportPath(path.join(pluginDir, "package.json"), path.join(rootDir, "scripts", "build-go-wasm-plugin.js"));
  const verifyScript = toImportPath(path.join(pluginDir, "package.json"), path.join(rootDir, "scripts", "verify-plugin-dev-workflow.js"));

  writeFile(
    path.join(pluginDir, "manifest.yaml"),
    [
      "apiVersion: agentforge/v1",
      "kind: WorkflowPlugin",
      "metadata:",
      `  id: ${pluginId}`,
      `  name: ${displayName}`,
      `  version: ${version}`,
      `  description: Starter workflow plugin for ${displayName}.`,
      "spec:",
      "  runtime: wasm",
      `  module: ./dist/${name}.wasm`,
      "  abiVersion: v1",
      "  capabilities:",
      "    - run_workflow",
      "  workflow:",
      "    process: sequential",
      "    roles:",
      "      - id: coder",
      "      - id: reviewer",
      "    steps:",
      "      - id: implement",
      "        role: coder",
      "        action: agent",
      "        next: [review]",
      "      - id: review",
      "        role: reviewer",
      "        action: review",
      "    triggers:",
      "      - event: manual",
      "permissions: {}",
      "source:",
      "  type: local",
      `  path: ./plugins/workflows/${name}/manifest.yaml`,
      "",
    ].join("\n"),
  );

  writeJson(path.join(pluginDir, "package.json"), {
    name: `@agentforge/workflow-${name}`,
    private: true,
    scripts: {
      "plugin:build": `node ${buildScript} --manifest plugins/workflows/${name}/manifest.yaml --source ./cmd/${name}`,
      "plugin:verify": `node ${verifyScript} --manifest plugins/workflows/${name}/manifest.yaml`,
    },
  });

  writeFile(
    path.join(goSourceDir, "main.go"),
    `package main

import (
  "fmt"

  pluginsdk "${modulePath}/plugin-sdk-go"
)

type workflowPlugin struct{}

func (workflowPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
  return &pluginsdk.Descriptor{
    APIVersion: "agentforge/v1",
    Kind:       "WorkflowPlugin",
    ID:         "${pluginId}",
    Name:       "${displayName}",
    Version:    "${version}",
    Runtime:    "wasm",
    ABIVersion: pluginsdk.ABIVersion,
    Description: "Starter workflow plugin for ${displayName}.",
    Capabilities: []pluginsdk.Capability{
      {Name: "run_workflow", Description: "Execute the starter sequential workflow"},
    },
  }, nil
}

func (workflowPlugin) Init(ctx *pluginsdk.Context) error {
  return nil
}

func (workflowPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
  return pluginsdk.Success(map[string]any{
    "status": "ok",
    "workflow": "${pluginId}",
  }), nil
}

func (workflowPlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
  switch invocation.Operation {
  case "run_workflow":
    return pluginsdk.Success(map[string]any{
      "status": "accepted",
      "workflow": "${pluginId}",
      "operation": invocation.Operation,
    }), nil
  default:
    return nil, pluginsdk.NewRuntimeError("unsupported_operation", fmt.Sprintf("unsupported operation %s", invocation.Operation)).
      WithDetail("operation", invocation.Operation)
  }
}

var runtime = pluginsdk.NewRuntime(workflowPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 {
  return pluginsdk.ExportABIVersion(runtime)
}

//go:wasmexport agentforge_run
func agentforgeRun() uint32 {
  return pluginsdk.ExportRun(runtime)
}

func main() {
  pluginsdk.Autorun(runtime)
}
`,
  );

  writeFile(
    path.join(goSourceDir, "main_test.go"),
    `package main

import "testing"

func TestDescribeExposesWorkflowMetadata(t *testing.T) {
  descriptor, err := workflowPlugin{}.Describe(nil)
  if err != nil {
    t.Fatalf("describe plugin: %v", err)
  }
  if descriptor.Kind != "WorkflowPlugin" {
    t.Fatalf("expected workflow kind, got %s", descriptor.Kind)
  }
  if descriptor.ID != "${pluginId}" {
    t.Fatalf("expected plugin id ${pluginId}, got %s", descriptor.ID)
  }
}
`,
  );

  return { pluginDir, goSourceDir };
}

function scaffoldIntegrationPlugin({ rootDir, name, displayName, pluginId, version, modulePath }) {
  const pluginDir = path.join(rootDir, "plugins", "integrations", name);
  const goSourceDir = path.join(rootDir, "src-go", "cmd", name);
  const buildScript = toImportPath(path.join(pluginDir, "package.json"), path.join(rootDir, "scripts", "build-go-wasm-plugin.js"));
  const verifyScript = toImportPath(path.join(pluginDir, "package.json"), path.join(rootDir, "scripts", "verify-plugin-dev-workflow.js"));

  writeFile(
    path.join(pluginDir, "manifest.yaml"),
    [
      "apiVersion: agentforge/v1",
      "kind: IntegrationPlugin",
      "metadata:",
      `  id: ${pluginId}`,
      `  name: ${displayName}`,
      `  version: ${version}`,
      `  description: Starter integration plugin for ${displayName}.`,
      "spec:",
      "  runtime: wasm",
      `  module: ./dist/${name}.wasm`,
      "  abiVersion: v1",
      "  capabilities:",
      "    - health",
      "    - send_message",
      "permissions: {}",
      "source:",
      "  type: local",
      `  path: ./plugins/integrations/${name}/manifest.yaml`,
      "",
    ].join("\n"),
  );

  writeJson(path.join(pluginDir, "package.json"), {
    name: `@agentforge/integration-${name}`,
    private: true,
    scripts: {
      "plugin:build": `node ${buildScript} --manifest plugins/integrations/${name}/manifest.yaml --source ./cmd/${name}`,
      "plugin:verify": `node ${verifyScript} --manifest plugins/integrations/${name}/manifest.yaml`,
    },
  });

  writeFile(
    path.join(goSourceDir, "main.go"),
    `package main

import (
  "fmt"

  pluginsdk "${modulePath}/plugin-sdk-go"
)

type integrationPlugin struct{}

func (integrationPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
  return &pluginsdk.Descriptor{
    APIVersion: "agentforge/v1",
    Kind:       "IntegrationPlugin",
    ID:         "${pluginId}",
    Name:       "${displayName}",
    Version:    "${version}",
    Runtime:    "wasm",
    ABIVersion: pluginsdk.ABIVersion,
    Description: "Starter integration plugin for ${displayName}.",
    Capabilities: []pluginsdk.Capability{
      {Name: "health", Description: "Report plugin health"},
      {Name: "send_message", Description: "Send a starter message"},
    },
  }, nil
}

func (integrationPlugin) Init(ctx *pluginsdk.Context) error {
  return nil
}

func (integrationPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
  return pluginsdk.Success(map[string]any{"status": "ok"}), nil
}

func (integrationPlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
  switch invocation.Operation {
  case "send_message":
    return pluginsdk.Success(map[string]any{
      "status": "sent",
      "target": invocation.Payload["target"],
    }), nil
  default:
    return nil, pluginsdk.NewRuntimeError("unsupported_operation", fmt.Sprintf("unsupported operation %s", invocation.Operation)).
      WithDetail("operation", invocation.Operation)
  }
}

var runtime = pluginsdk.NewRuntime(integrationPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 {
  return pluginsdk.ExportABIVersion(runtime)
}

//go:wasmexport agentforge_run
func agentforgeRun() uint32 {
  return pluginsdk.ExportRun(runtime)
}

func main() {
  pluginsdk.Autorun(runtime)
}
`,
  );

  writeFile(
    path.join(goSourceDir, "main_test.go"),
    `package main

import "testing"

func TestDescribeExposesIntegrationMetadata(t *testing.T) {
  descriptor, err := integrationPlugin{}.Describe(nil)
  if err != nil {
    t.Fatalf("describe plugin: %v", err)
  }
  if descriptor.Kind != "IntegrationPlugin" {
    t.Fatalf("expected integration kind, got %s", descriptor.Kind)
  }
}
`,
  );

  return { pluginDir, goSourceDir };
}

function main(argv = process.argv.slice(2)) {
  const parsed = parseArgs(argv);
  const result = scaffoldPlugin(parsed);
  console.log(`Created ${parsed.type} plugin scaffold at ${result.pluginDir}`);
  if (result.goSourceDir) {
    console.log(`Created Go entrypoint at ${result.goSourceDir}`);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
  parseArgs,
  scaffoldPlugin,
};
