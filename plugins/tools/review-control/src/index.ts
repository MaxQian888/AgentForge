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
  id: "review-control",
  name: "Review Control",
  version: "0.1.0",
  description: "Built-in MCP control tool for AgentForge review trigger, detail, and task review history workflows.",
  tags: ["builtin", "tool", "reviews", "control-plane"],
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  source: {
    type: "builtin",
    path: "./plugins/tools/review-control/manifest.yaml",
  },
});

export const plugin = defineToolPlugin({
  manifest,
  tools: [
    {
      name: "review:trigger",
      title: "Trigger Review",
      description: "Trigger a review through the AgentForge review control-plane endpoint.",
      execute: async (args) => {
        const prUrl = requireString(args, "prUrl");
        const body = {
          taskId: String(args.taskId ?? "").trim() || undefined,
          projectId: String(args.projectId ?? "").trim() || undefined,
          prUrl,
          prNumber: Number(args.prNumber ?? 0) || 0,
          trigger: String(args.trigger ?? "manual"),
          event: String(args.event ?? ""),
          dimensions: Array.isArray(args.dimensions) ? args.dimensions : [],
          changedFiles: Array.isArray(args.changedFiles) ? args.changedFiles : [],
          diff: String(args.diff ?? ""),
        };
        const review = await requestControlPlaneJson<JsonRecord>("/api/v1/reviews/trigger", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(body),
        });
        return {
          content: [{ type: "text", text: `Triggered review ${String(review.id ?? "unknown")}.` }],
          structuredContent: { review },
          isError: false,
        };
      },
    },
    {
      name: "review:get",
      title: "Get Review",
      description: "Fetch review detail from the AgentForge review control plane.",
      execute: async (args) => {
        const reviewId = requireString(args, "reviewId");
        const review = await requestControlPlaneJson<JsonRecord>(`/api/v1/reviews/${reviewId}`, {
          method: "GET",
        });
        return {
          content: [{ type: "text", text: `Loaded review ${String(review.id ?? reviewId)}.` }],
          structuredContent: { review },
          isError: false,
        };
      },
    },
    {
      name: "review:list-by-task",
      title: "List Reviews By Task",
      description: "List reviews attached to a task through the AgentForge task review surface.",
      execute: async (args) => {
        const taskId = requireString(args, "taskId");
        const reviews = await requestControlPlaneJson<JsonRecord[]>(`/api/v1/tasks/${taskId}/reviews`, {
          method: "GET",
        });
        return {
          content: [{ type: "text", text: `Loaded ${reviews.length} reviews for task ${taskId}.` }],
          structuredContent: { reviews },
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
