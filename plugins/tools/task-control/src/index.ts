import {
  createPluginMcpServer,
  createToolPluginManifest,
  defineToolPlugin,
  connectPluginStdioServer,
} from "../../../../src-bridge/src/plugin-sdk/index.js";
import { requestControlPlaneJson, type JsonRecord } from "../../_shared/control-plane.js";

function requireTaskId(args: JsonRecord): string {
  const taskId = String(args.taskId ?? "").trim();
  if (!taskId) {
    throw new Error("taskId is required");
  }
  return taskId;
}

export const manifest = createToolPluginManifest({
  id: "task-control",
  name: "Task Control",
  version: "0.1.0",
  description: "Built-in MCP control tool for AgentForge task lookup, decomposition, and dispatch observability.",
  tags: ["builtin", "tool", "tasks", "control-plane"],
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  source: {
    type: "builtin",
    path: "./plugins/tools/task-control/manifest.yaml",
  },
});

export const plugin = defineToolPlugin({
  manifest,
  tools: [
    {
      name: "task:get",
      title: "Get Task",
      description: "Fetch task detail from the AgentForge task control plane.",
      execute: async (args) => {
        const taskId = requireTaskId(args);
        const task = await requestControlPlaneJson<JsonRecord>(`/api/v1/tasks/${taskId}`, {
          method: "GET",
        });
        return {
          content: [{ type: "text", text: `Loaded task ${String(task["id"] ?? taskId)}.` }],
          structuredContent: { task },
          isError: false,
        };
      },
    },
    {
      name: "task:decompose",
      title: "Decompose Task",
      description: "Run the existing task decomposition endpoint for a task.",
      execute: async (args) => {
        const taskId = requireTaskId(args);
        const decomposition = await requestControlPlaneJson<JsonRecord>(
          `/api/v1/tasks/${taskId}/decompose`,
          {
            method: "POST",
          },
        );
        return {
          content: [
            {
              type: "text",
              text: `Decomposed task ${String(taskId)} into ${Array.isArray(decomposition["subtasks"]) ? decomposition["subtasks"].length : 0} subtasks.`,
            },
          ],
          structuredContent: { decomposition },
          isError: false,
        };
      },
    },
    {
      name: "task:dispatch-history",
      title: "Get Task Dispatch History",
      description: "Read dispatch history for a task from the AgentForge dispatch observability surface.",
      execute: async (args) => {
        const taskId = requireTaskId(args);
        const history = await requestControlPlaneJson<JsonRecord[]>(
          `/api/v1/tasks/${taskId}/dispatch/history`,
          {
            method: "GET",
          },
        );
        return {
          content: [
            {
              type: "text",
              text: `Loaded ${history.length} dispatch history entries for task ${String(taskId)}.`,
            },
          ],
          structuredContent: { history },
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
