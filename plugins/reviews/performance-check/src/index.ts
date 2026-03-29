import {
  connectPluginStdioServer,
  createPluginMcpServer,
  createReviewPluginManifest,
  createReviewResult,
  defineReviewPlugin,
} from "../../../../src-bridge/src/plugin-sdk/index.js";

export const manifest = createReviewPluginManifest({
  id: "performance-check",
  name: "Performance Check",
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
      pluginId: "performance-check",
      summary: `Performance review ${String(review_id)} flagged one hot path concern.`,
      findings: [
        {
          category: "performance",
          severity: "medium",
          file: "src-bridge/src/review/orchestrator.ts",
          line: 1,
          message: "Parallel review fan-out should avoid avoidable duplicate work on large pull requests.",
        },
      ],
    }),
});

if (import.meta.main) {
  const server = createPluginMcpServer(plugin);
  await connectPluginStdioServer(server);
}
