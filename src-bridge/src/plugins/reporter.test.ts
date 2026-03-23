import { afterEach, describe, expect, test } from "bun:test";
import {
  createDefaultPluginRuntimeReporter,
  HttpPluginRuntimeReporter,
} from "./reporter.js";
import type { PluginRuntimeUpdate } from "./types.js";

const update: PluginRuntimeUpdate = {
  plugin_id: "tool.alpha",
  host: "ts-bridge",
  lifecycle_state: "active",
  last_health_at: "2026-03-23T00:00:00Z",
  restart_count: 2,
};

const originalFetch = globalThis.fetch;
const originalEndpoint = process.env.GO_PLUGIN_RUNTIME_SYNC_URL;

afterEach(() => {
  globalThis.fetch = originalFetch;
  if (originalEndpoint === undefined) {
    delete process.env.GO_PLUGIN_RUNTIME_SYNC_URL;
    return;
  }
  process.env.GO_PLUGIN_RUNTIME_SYNC_URL = originalEndpoint;
});

describe("plugin runtime reporter", () => {
  test("returns a noop reporter when no endpoint is configured", async () => {
    delete process.env.GO_PLUGIN_RUNTIME_SYNC_URL;

    const reporter = createDefaultPluginRuntimeReporter();

    await expect(reporter.report(update)).resolves.toBeUndefined();
  });

  test("posts runtime updates to the configured endpoint", async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
    process.env.GO_PLUGIN_RUNTIME_SYNC_URL = "https://example.com/runtime";
    globalThis.fetch = (async (input, init) => {
      calls.push({ input, init });
      return { ok: true, status: 204 } as Response;
    }) as typeof fetch;

    const reporter = createDefaultPluginRuntimeReporter();
    await reporter.report(update);

    expect(calls).toHaveLength(1);
    expect(calls[0]?.input).toBe("https://example.com/runtime");
    expect(calls[0]?.init).toMatchObject({
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify(update),
    });
  });

  test("throws when the remote report endpoint responds with an error", async () => {
    globalThis.fetch = ((async () => {
      return { ok: false, status: 500 } as Response;
    }) as unknown) as typeof fetch;

    const reporter = new HttpPluginRuntimeReporter("https://example.com/runtime");

    await expect(reporter.report(update)).rejects.toThrow(
      "Plugin runtime report failed with status 500",
    );
  });
});
