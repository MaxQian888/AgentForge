import {
  createPluginMcpServer,
  createToolPluginManifest,
  defineToolPlugin,
  connectPluginStdioServer,
} from "../../../../src-bridge/src/plugin-sdk/index.js";
import { requestControlPlaneJson, type JsonRecord } from "../../_shared/control-plane.js";

function requireString(args: JsonRecord, field: string): string {
  const value = String(args[field] ?? "").trim();
  if (!value) {
    throw new Error(`${field} is required`);
  }
  return value;
}

export const manifest = createToolPluginManifest({
  id: "workflow-control",
  name: "Workflow Control",
  version: "0.1.0",
  description: "Built-in MCP control tool for AgentForge workflow run start and run history inspection.",
  tags: ["builtin", "tool", "workflow", "control-plane"],
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  source: {
    type: "builtin",
    path: "./plugins/tools/workflow-control/manifest.yaml",
  },
});

export const plugin = defineToolPlugin({
  manifest,
  tools: [
    {
      name: "workflow:start",
      title: "Start Workflow Run",
      description: "Start a workflow run through the AgentForge workflow runtime surface.",
      execute: async (args) => {
        const pluginId = requireString(args, "pluginId");
        const trigger = (args.trigger ?? {}) as Record<string, unknown>;
        const run = await requestControlPlaneJson<JsonRecord>(
          `/api/v1/plugins/${pluginId}/workflow-runs`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify({ trigger }),
          },
        );
        return {
          content: [{ type: "text", text: `Started workflow run ${String(run.id ?? "unknown")}.` }],
          structuredContent: { run },
          isError: false,
        };
      },
    },
    {
      name: "workflow:get-run",
      title: "Get Workflow Run",
      description: "Fetch workflow run detail through the AgentForge workflow run endpoint.",
      execute: async (args) => {
        const runId = requireString(args, "runId");
        const run = await requestControlPlaneJson<JsonRecord>(
          `/api/v1/plugins/workflow-runs/${runId}`,
          {
            method: "GET",
          },
        );
        return {
          content: [{ type: "text", text: `Loaded workflow run ${String(run.id ?? runId)}.` }],
          structuredContent: { run },
          isError: false,
        };
      },
    },
    {
      name: "workflow:list-runs",
      title: "List Workflow Runs",
      description: "List workflow runs for a workflow plugin through the AgentForge workflow history endpoint.",
      execute: async (args) => {
        const pluginId = requireString(args, "pluginId");
        const runs = await requestControlPlaneJson<JsonRecord[]>(
          `/api/v1/plugins/${pluginId}/workflow-runs`,
          {
            method: "GET",
          },
        );
        return {
          content: [{ type: "text", text: `Loaded ${runs.length} workflow runs for ${pluginId}.` }],
          structuredContent: { runs },
          isError: false,
        };
      },
    },
  ] as const,
});

if (import.meta.main) {
  const server = createPluginMcpServer(plugin);
  await connectPluginStdioServer(server);
}
