import { describe, expect, test } from "bun:test";
import { z } from "zod";
import {
  createLocalPluginHarness,
  createReviewPluginManifest,
  createReviewResult,
  createToolPluginManifest,
  defineReviewPlugin,
  defineToolPlugin,
} from "./index.js";

function firstTextContent(result: { content: Array<{ type: string; text?: string }> }): string | undefined {
  const item = result.content.find((entry) => entry.type === "text");
  return typeof item?.text === "string" ? item.text : undefined;
}

describe("plugin SDK local harness", () => {
  test("executes tool plugin handlers without handwritten MCP glue", async () => {
    const plugin = defineToolPlugin({
      manifest: createToolPluginManifest({
        id: "tool.echo",
        name: "Echo Tool",
        version: "1.0.0",
        transport: "stdio",
        command: "node",
        args: ["dist/index.js"],
      }),
      tools: [
        {
          name: "echo",
          description: "Echo text back to the caller.",
          inputSchema: z.object({ text: z.string() }),
          execute: ({ text }) => ({
            content: [{ type: "text", text: String(text) }],
          }),
        },
      ],
    });

    const harness = createLocalPluginHarness(plugin);

    expect(harness.listTools()).toEqual([
      expect.objectContaining({ name: "echo" }),
    ]);

    const result = await harness.callTool("echo", { text: "hello" });
    expect(firstTextContent(result)).toBe("hello");
  });

  test("executes review plugin entrypoints with normalized findings output", async () => {
    const plugin = defineReviewPlugin({
      manifest: createReviewPluginManifest({
        id: "review.typescript",
        name: "TypeScript Review",
        version: "1.0.0",
        transport: "stdio",
        command: "bun",
        args: ["run", "src/index.ts"],
        triggers: {
          events: ["pull_request.updated"],
          filePatterns: ["src/**/*.ts"],
        },
      }),
      executeReview: ({ review_id }) =>
        createReviewResult({
          pluginId: "review.typescript",
          summary: `Review ${String(review_id)} produced one finding.`,
          findings: [
            {
              category: "typescript",
              severity: "low",
              file: "src/index.ts",
              line: 3,
              message: "Avoid implicit any in starter templates.",
            },
          ],
        }),
    });

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:run", { review_id: "review-123" });

    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        format: "findings/v1",
        findings: [
          expect.objectContaining({
            category: "typescript",
            sources: ["review.typescript"],
          }),
        ],
      }),
    );
  });
});
