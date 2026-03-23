import { afterEach, describe, expect, test } from "bun:test";
import { ToolPluginManager } from "./tool-plugin-manager.js";
import type { PluginManifest, PluginRuntimeReporter } from "./types.js";

const reporterCalls: unknown[] = [];

const reporter: PluginRuntimeReporter = {
  async report(update) {
    reporterCalls.push(update);
  },
};

const validManifest: PluginManifest = {
  apiVersion: "agentforge/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "repo-search",
    name: "Repo Search",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    transport: "stdio",
    command: process.execPath,
    args: ["-e", "setInterval(() => {}, 1000)"],
  },
  permissions: {},
  source: {
    type: "local",
    path: "./plugins/repo-search/manifest.yaml",
  },
};

let manager: ToolPluginManager | undefined;

afterEach(async () => {
  reporterCalls.length = 0;
  if (manager) {
    await manager.dispose();
    manager = undefined;
  }
});

describe("tool plugin manager", () => {
  test("registers a valid tool plugin manifest", async () => {
    manager = new ToolPluginManager({ reporter });

    const record = await manager.register(validManifest);

    expect(record.metadata.id).toBe("repo-search");
    expect(record.lifecycle_state).toBe("installed");
  });

  test("rejects unsupported kind and runtime combinations", async () => {
    manager = new ToolPluginManager({ reporter });

    await expect(
      manager.register({
        ...validManifest,
        kind: "IntegrationPlugin",
      } as PluginManifest),
    ).rejects.toThrow("kind");
  });

  test("refuses to activate a disabled plugin until re-enabled", async () => {
    manager = new ToolPluginManager({ reporter });
    await manager.register(validManifest);
    await manager.disable("repo-search");

    await expect(manager.activate("repo-search")).rejects.toThrow("disabled");

    await manager.enable("repo-search");
    const status = await manager.activate("repo-search");
    expect(status.lifecycle_state).toBe("active");
  });

  test("reports activation and health transitions", async () => {
    manager = new ToolPluginManager({ reporter });
    await manager.register(validManifest);

    await manager.enable("repo-search");
    await manager.activate("repo-search");
    const health = await manager.checkHealth("repo-search");

    expect(health.lifecycle_state).toBe("active");
    expect(reporterCalls.length).toBeGreaterThanOrEqual(2);
  });
});
