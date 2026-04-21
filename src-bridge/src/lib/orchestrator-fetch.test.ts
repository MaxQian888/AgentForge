import { describe, it, expect } from "bun:test";
import { orchestratorFetch, ingestToOrchestrator, type FetchFn } from "./orchestrator-fetch.js";

describe("orchestratorFetch", () => {
  it("attaches X-Trace-ID from c.var.traceId", async () => {
    let captured: Headers | null = null;
    const fakeFetch: FetchFn = async (_input, init) => {
      captured = new Headers(init?.headers);
      return new Response("{}", { status: 200 });
    };
    const c = { var: { traceId: "tr_outbound0000000000000000" } };
    await orchestratorFetch(c, "/foo", {}, { fetch: fakeFetch, baseUrl: "http://orch.test" });
    expect(captured!.get("X-Trace-ID")).toBe("tr_outbound0000000000000000");
  });

  it("posts to the configured base URL", async () => {
    let url = "";
    const fakeFetch: FetchFn = async (input) => {
      url = typeof input === "string" ? input : input.toString();
      return new Response("{}", { status: 200 });
    };
    const c = { var: { traceId: "tr_a" } };
    await orchestratorFetch(c, "/health", {}, { fetch: fakeFetch, baseUrl: "http://orch.test" });
    expect(url).toBe("http://orch.test/health");
  });

  it("omits X-Trace-ID when traceId empty", async () => {
    let captured: Headers | null = null;
    const fakeFetch: FetchFn = async (_i, init) => {
      captured = new Headers(init?.headers);
      return new Response("{}", { status: 200 });
    };
    const c = { var: { traceId: "" } };
    await orchestratorFetch(c, "/x", {}, { fetch: fakeFetch, baseUrl: "http://o" });
    expect(captured!.get("X-Trace-ID")).toBeNull();
  });
});

describe("ingestToOrchestrator", () => {
  it("posts JSON to /api/v1/internal/logs/ingest", async () => {
    let url = "", body = "";
    const fakeFetch: FetchFn = async (input, init) => {
      url = typeof input === "string" ? input : input.toString();
      body = String(init?.body ?? "");
      return new Response(null, { status: 202 });
    };
    const c = { var: { traceId: "tr_ingest0000000000000000000" } };
    ingestToOrchestrator(c, { level: "warn", source: "ts-bridge", summary: "plugin load failed", detail: { plugin: "acme" } }, { fetch: fakeFetch, baseUrl: "http://orch.test" });
    // Give the microtask a chance to run
    await new Promise((r) => setTimeout(r, 10));
    expect(url).toBe("http://orch.test/api/v1/internal/logs/ingest");
    const parsed = JSON.parse(body);
    expect(parsed.level).toBe("warn");
    expect(parsed.summary).toBe("plugin load failed");
    expect(parsed.tab).toBe("system");
  });

  it("does not throw on fetch failure", async () => {
    const fakeFetch: FetchFn = async () => { throw new Error("offline"); };
    const c = { var: { traceId: "" } };
    ingestToOrchestrator(c, { level: "error", source: "ts-bridge", summary: "boom" }, { fetch: fakeFetch, baseUrl: "http://o" });
    await new Promise((r) => setTimeout(r, 10));
    // If we got here without exception, test passes
  });
});
