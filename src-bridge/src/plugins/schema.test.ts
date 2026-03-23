import { describe, expect, test } from "bun:test";
import {
  PluginManifestSchema,
  PluginRegisterRequestSchema,
  ToolPluginManifestSchema,
} from "./schema.js";

const validToolPlugin = {
  apiVersion: "agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "tool.plugin",
    name: "Tool Plugin",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    transport: "stdio",
    command: "node",
    args: ["plugin.js"],
  },
};

describe("plugin schemas", () => {
  test("parses a valid tool plugin manifest and defaults permissions", () => {
    const parsed = PluginRegisterRequestSchema.parse({
      manifest: validToolPlugin,
    });

    expect(parsed.manifest.permissions).toEqual({});
    expect(parsed.manifest.source).toBeUndefined();
  });

  test("rejects tool plugins that do not use the mcp runtime or omit stdio command", () => {
    const invalidRuntime = ToolPluginManifestSchema.safeParse({
      ...validToolPlugin,
      spec: {
        ...validToolPlugin.spec,
        runtime: "declarative",
      },
    });

    const missingCommand = PluginManifestSchema.safeParse({
      ...validToolPlugin,
      spec: {
        ...validToolPlugin.spec,
        command: undefined,
      },
    });

    expect(invalidRuntime.success).toBe(false);
    expect(JSON.stringify(invalidRuntime.error?.issues)).toContain(
      "ToolPlugin manifests must use the mcp runtime",
    );
    expect(missingCommand.success).toBe(false);
    expect(JSON.stringify(missingCommand.error?.issues)).toContain(
      "stdio tool plugins must define spec.command",
    );
  });

  test("rejects integration plugins that do not use the go-plugin runtime", () => {
    const invalidIntegration = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "IntegrationPlugin",
      metadata: {
        id: "integration.plugin",
        name: "Integration Plugin",
        version: "1.0.0",
      },
      spec: {
        runtime: "mcp",
      },
    });

    expect(invalidIntegration.success).toBe(false);
    expect(JSON.stringify(invalidIntegration.error?.issues)).toContain(
      "IntegrationPlugin manifests must use the go-plugin runtime",
    );
  });
});
