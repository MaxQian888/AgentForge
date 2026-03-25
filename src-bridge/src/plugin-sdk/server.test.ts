import { describe, expect, test } from "bun:test";
import { z } from "zod";
import {
  connectPluginStdioServer,
  createPluginMcpServer,
  defineReviewPlugin,
  defineToolPlugin,
  listPluginToolDefinitions,
} from "./server.js";
import { createReviewPluginManifest, createToolPluginManifest } from "./manifest.js";

describe("plugin SDK server helpers", () => {
  test("lists tool plugin definitions verbatim and registers them with the MCP server", async () => {
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
          title: "Echo",
          description: "Echo text back to the caller.",
          inputSchema: z.object({ text: z.string() }),
          execute: ({ text }) => ({
            content: [{ type: "text", text }],
          }),
        },
      ] as const,
    });

    const listed = listPluginToolDefinitions(plugin);
    expect(listed).toHaveLength(1);
    expect(listed[0]).toMatchObject({
      name: "echo",
      title: "Echo",
      description: "Echo text back to the caller.",
    });

    const server = createPluginMcpServer(plugin) as unknown as {
      _registeredTools: Record<
        string,
        {
          title?: string;
          description?: string;
          handler: (args: Record<string, unknown>) => Promise<{ content: Array<{ text: string }> }>;
        }
      >;
    };
    expect(Object.keys(server._registeredTools)).toEqual(["echo"]);
    expect(server._registeredTools.echo.title).toBe("Echo");
    expect(server._registeredTools.echo.description).toBe("Echo text back to the caller.");
    await expect(server._registeredTools.echo.handler({ text: "hello" })).resolves.toEqual({
      content: [{ type: "text", text: "hello" }],
    });
  });

  test("derives a default review entrypoint, description, and fallback input schema", async () => {
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
        },
      }),
      executeReview: ({ review_id }) => ({
        content: [{ type: "text", text: `review ${String(review_id)}` }],
      }),
    });

    const listed = listPluginToolDefinitions(plugin);
    expect(listed).toHaveLength(1);
    expect(listed[0]).toMatchObject({
      name: "review:run",
      title: "TypeScript Review",
      description: "Run review plugin analysis.",
    });
    expect(listed[0].inputSchema?.parse({ review_id: "review-123" })).toEqual({
      review_id: "review-123",
    });
    expect(await listed[0].execute({ review_id: "review-123" })).toEqual({
      content: [{ type: "text", text: "review review-123" }],
    });
  });

  test("connects a plugin MCP server over stdio using the provided server instance", async () => {
    let receivedTransport: unknown;
    const fakeServer = {
      async connect(transport: unknown) {
        receivedTransport = transport;
      },
    };

    const connected = await connectPluginStdioServer(fakeServer as never);

    expect(connected).toBe(fakeServer);
    expect(receivedTransport).toBeDefined();
    expect((receivedTransport as { constructor?: { name?: string } }).constructor?.name).toBe(
      "StdioServerTransport",
    );
  });
});
