import { describe, expect, test } from "bun:test";
import { createReviewPluginManifest, createToolPluginManifest } from "./index.js";
import { PluginManifestSchema } from "../plugins/schema.js";

describe("plugin SDK manifest helpers", () => {
  test("creates a valid tool plugin manifest with MCP defaults", () => {
    const manifest = createToolPluginManifest({
      id: "tool.echo",
      name: "Echo Tool",
      version: "1.0.0",
      transport: "stdio",
      command: "node",
      args: ["dist/index.js"],
      capabilities: ["echo"],
    });

    const parsed = PluginManifestSchema.parse(manifest);
    expect(parsed.kind).toBe("ToolPlugin");
    expect(parsed.spec.runtime).toBe("mcp");
    expect(parsed.permissions).toEqual({});
    expect(parsed.spec.command).toBe("node");
  });

  test("creates a valid review plugin manifest with findings output", () => {
    const manifest = createReviewPluginManifest({
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
    });

    const parsed = PluginManifestSchema.parse(manifest);
    expect(parsed.kind).toBe("ReviewPlugin");
    expect(parsed.spec.review?.entrypoint).toBe("review:run");
    expect(parsed.spec.review?.output.format).toBe("findings/v1");
  });
});
