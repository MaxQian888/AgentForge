import {
  connectPluginStdioServer,
  createPluginMcpServer,
  createReviewPluginManifest,
  createReviewResult,
  defineReviewPlugin,
} from "../../../../src-bridge/src/plugin-sdk/index.js";

export const manifest = createReviewPluginManifest({
  id: "architecture-check",
  name: "Architecture Check",
  version: "0.1.0",
  transport: "stdio",
  command: "bun",
  args: ["run", "src/index.ts"],
  triggers: {
    events: ["pull_request.updated"],
    filePatterns: ["src/**/*.ts", "src-go/**/*.go"],
  },
});

export const plugin = defineReviewPlugin({
  manifest,
  executeReview: async ({ review_id }) =>
    createReviewResult({
      pluginId: "architecture-check",
      summary: `Architecture review ${String(review_id)} flagged one boundary concern.`,
      findings: [
        {
          category: "architecture",
          severity: "medium",
          file: "src-go/internal/handler/plugin_handler.go",
          line: 1,
          message: "Handler logic should keep orchestration decisions behind the approved service boundary.",
        },
      ],
    }),
});

if (import.meta.main) {
  const server = createPluginMcpServer(plugin);
  await connectPluginStdioServer(server);
}
