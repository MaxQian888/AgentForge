import { describe, it, expect } from "bun:test";
import { Hono } from "hono";
import { traceMiddleware } from "./trace.js";

describe("traceMiddleware", () => {
  it("uses inbound X-Trace-ID when present", async () => {
    const app = new Hono();
    app.use("*", traceMiddleware());
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/", { headers: { "X-Trace-ID": "tr_inbound" } });
    expect(await res.text()).toBe("tr_inbound");
    expect(res.headers.get("X-Trace-ID")).toBe("tr_inbound");
  });

  it("generates a trace_id when missing", async () => {
    const app = new Hono();
    app.use("*", traceMiddleware());
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/");
    const body = await res.text();
    expect(body.startsWith("tr_")).toBe(true);
    expect(body.length).toBe(27);
    expect(res.headers.get("X-Trace-ID")).toBe(body);
  });

  it("falls back to X-Request-ID when X-Trace-ID missing", async () => {
    const app = new Hono();
    app.use("*", traceMiddleware());
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/", { headers: { "X-Request-ID": "req-abc" } });
    expect(await res.text()).toBe("req-abc");
  });

  it("invokes onMidchain when generated", async () => {
    const seen: string[] = [];
    const app = new Hono();
    app.use("*", traceMiddleware({ onMidchain: (id) => seen.push(id) }));
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/");
    const body = await res.text();
    expect(seen).toEqual([body]);
  });

  it("does not invoke onMidchain when inbound trace supplied", async () => {
    const seen: string[] = [];
    const app = new Hono();
    app.use("*", traceMiddleware({ onMidchain: (id) => seen.push(id) }));
    app.get("/", (c) => c.text(""));
    await app.request("/", { headers: { "X-Trace-ID": "tr_inbound" } });
    expect(seen).toEqual([]);
  });
});
