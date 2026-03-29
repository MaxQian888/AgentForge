import { afterEach, describe, expect, test } from "bun:test";

const originalFetch = globalThis.fetch;
const originalServerUrl = process.env.OPENCODE_SERVER_URL;
const originalUsername = process.env.OPENCODE_SERVER_USERNAME;
const originalPassword = process.env.OPENCODE_SERVER_PASSWORD;
const originalModel = process.env.OPENCODE_RUNTIME_MODEL;

afterEach(() => {
  globalThis.fetch = originalFetch;

  if (originalServerUrl === undefined) {
    delete process.env.OPENCODE_SERVER_URL;
  } else {
    process.env.OPENCODE_SERVER_URL = originalServerUrl;
  }

  if (originalUsername === undefined) {
    delete process.env.OPENCODE_SERVER_USERNAME;
  } else {
    process.env.OPENCODE_SERVER_USERNAME = originalUsername;
  }

  if (originalPassword === undefined) {
    delete process.env.OPENCODE_SERVER_PASSWORD;
  } else {
    process.env.OPENCODE_SERVER_PASSWORD = originalPassword;
  }

  if (originalModel === undefined) {
    delete process.env.OPENCODE_RUNTIME_MODEL;
  } else {
    process.env.OPENCODE_RUNTIME_MODEL = originalModel;
  }
});

describe("OpenCode transport", () => {
  test("probes health and provider/model readiness through the configured OpenCode server", async () => {
    process.env.OPENCODE_SERVER_URL = "http://127.0.0.1:4096";
    process.env.OPENCODE_RUNTIME_MODEL = "opencode/gpt-oss";

    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
    globalThis.fetch = (async (input, init) => {
      calls.push({ input, init });
      const url = String(input);
      if (url.endsWith("/global/health")) {
        return Response.json({ healthy: true, version: "0.1.0" });
      }
      if (url.endsWith("/provider")) {
        return Response.json({
          all: [{ id: "opencode", name: "OpenCode" }],
          connected: ["opencode"],
        });
      }
      if (url.endsWith("/config/providers")) {
        return Response.json({
          default: { opencode: "opencode/gpt-oss" },
          providers: [{ id: "opencode", models: [{ id: "opencode/gpt-oss" }] }],
        });
      }
      throw new Error(`Unexpected fetch ${url}`);
    }) as typeof fetch;

    const mod = await import("./transport.js");
    const transport = mod.createOpenCodeTransport();
    const readiness = await transport.checkReadiness({
      provider: "opencode",
      model: "opencode/gpt-oss",
    });

    expect(readiness.ok).toBe(true);
    expect(readiness.version).toBe("0.1.0");
    expect(calls.map((call) => String(call.input))).toEqual([
      "http://127.0.0.1:4096/global/health",
      "http://127.0.0.1:4096/provider",
      "http://127.0.0.1:4096/config/providers",
    ]);
  });

  test("includes basic auth when server credentials are configured", async () => {
    process.env.OPENCODE_SERVER_URL = "http://127.0.0.1:4096";
    process.env.OPENCODE_SERVER_USERNAME = "bridge";
    process.env.OPENCODE_SERVER_PASSWORD = "secret";

    const authHeaders: string[] = [];
    globalThis.fetch = (async (_input, init) => {
      const headers = new Headers(init?.headers);
      authHeaders.push(headers.get("authorization") ?? "");
      return Response.json({ healthy: true, version: "0.1.0" });
    }) as typeof fetch;

    const mod = await import("./transport.js");
    const transport = mod.createOpenCodeTransport();
    await transport.getHealth();

    expect(authHeaders).toEqual(["Basic YnJpZGdlOnNlY3JldA=="]);
  });

  test("returns explicit blocking diagnostics when the server is unreachable or auth fails", async () => {
    process.env.OPENCODE_SERVER_URL = "http://127.0.0.1:4096";

    const mod = await import("./transport.js");
    const transport = mod.createOpenCodeTransport({
      fetch: (async () => {
        throw new Error("connect ECONNREFUSED 127.0.0.1:4096");
      }) as unknown as typeof fetch,
    });

    await expect(
      transport.checkReadiness({
        provider: "opencode",
        model: "opencode/gpt-oss",
      }),
    ).resolves.toMatchObject({
      ok: false,
      diagnostics: expect.arrayContaining([
        expect.objectContaining({ code: "server_unreachable", blocking: true }),
      ]),
    });

    const unauthorizedTransport = mod.createOpenCodeTransport({
      fetch: (async () => new Response("unauthorized", { status: 401 })) as unknown as typeof fetch,
    });

    await expect(
      unauthorizedTransport.checkReadiness({
        provider: "opencode",
        model: "opencode/gpt-oss",
      }),
    ).resolves.toMatchObject({
      ok: false,
      diagnostics: expect.arrayContaining([
        expect.objectContaining({ code: "authentication_failed", blocking: true }),
      ]),
    });
  });

  test("creates sessions, sends prompts asynchronously, and aborts the upstream session through the official APIs", async () => {
    process.env.OPENCODE_SERVER_URL = "http://127.0.0.1:4096";

    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
    globalThis.fetch = (async (input, init) => {
      calls.push({ input, init });
      const url = String(input);
      if (url.endsWith("/session") && init?.method === "POST") {
        return Response.json({ id: "session-123" });
      }
      if (url.endsWith("/session/session-123/prompt_async") && init?.method === "POST") {
        return new Response(null, { status: 204 });
      }
      if (url.endsWith("/session/session-123/abort") && init?.method === "POST") {
        return Response.json(true);
      }
      throw new Error(`Unexpected fetch ${url}`);
    }) as typeof fetch;

    const mod = await import("./transport.js");
    const transport = mod.createOpenCodeTransport();
    const session = await transport.createSession({ title: "AgentForge task" });
    await transport.sendPromptAsync({
      sessionId: session.id,
      provider: "opencode",
      model: "opencode/gpt-oss",
      prompt: "Continue the existing task",
    });
    const aborted = await transport.abortSession(session.id);

    expect(session).toEqual({ id: "session-123" });
    expect(aborted).toBe(true);
    expect(calls.map((call) => ({ input: String(call.input), method: call.init?.method ?? "GET" }))).toEqual([
      { input: "http://127.0.0.1:4096/session", method: "POST" },
      { input: "http://127.0.0.1:4096/session/session-123/prompt_async", method: "POST" },
      { input: "http://127.0.0.1:4096/session/session-123/abort", method: "POST" },
    ]);
    expect(JSON.parse(String(calls[1]?.init?.body))).toMatchObject({
      model: "opencode/gpt-oss",
      parts: [{ type: "text", text: "Continue the existing task" }],
    });
  });
});
