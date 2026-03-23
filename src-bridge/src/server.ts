import { Hono } from "hono";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { EventStreamer } from "./ws/event-stream.js";
import { handleExecute } from "./handlers/execute.js";
import { ExecuteRequestSchema, CancelRequestSchema } from "./schemas.js";
import type { HealthResponse } from "./types.js";

const app = new Hono();
const pool = new RuntimePoolManager(
  parseInt(process.env.MAX_CONCURRENT_AGENTS ?? "10"),
);
const streamer = new EventStreamer(
  process.env.GO_WS_URL ?? "ws://localhost:7777/ws/bridge",
);
const startTime = Date.now();

// Connect WebSocket to Go Orchestrator
streamer.connect();

// --- Routes ---

app.post("/bridge/execute", async (c) => {
  try {
    const body = await c.req.json();
    const parsed = ExecuteRequestSchema.safeParse(body);
    if (!parsed.success) {
      return c.json(
        { error: "Validation failed", details: parsed.error.flatten() },
        400,
      );
    }
    const result = await handleExecute(pool, streamer, parsed.data);
    return c.json(result, 200);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    if (message.includes("Pool at capacity")) {
      return c.json({ error: message }, 503);
    }
    if (message.includes("already exists")) {
      return c.json({ error: message }, 409);
    }
    return c.json({ error: message }, 500);
  }
});

app.get("/bridge/status/:id", (c) => {
  const runtime = pool.get(c.req.param("id"));
  if (!runtime) {
    return c.json({ error: "Not found" }, 404);
  }
  return c.json(runtime.toStatus());
});

app.post("/bridge/cancel", async (c) => {
  try {
    const body = await c.req.json();
    const parsed = CancelRequestSchema.safeParse(body);
    if (!parsed.success) {
      return c.json(
        { error: "Validation failed", details: parsed.error.flatten() },
        400,
      );
    }
    const runtime = pool.get(parsed.data.task_id);
    if (!runtime) {
      return c.json({ success: false, error: "Not found" }, 404);
    }
    runtime.cancel();
    pool.release(parsed.data.task_id);
    return c.json({ success: true });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    return c.json({ error: message }, 500);
  }
});

app.get("/bridge/health", (c) => {
  const stats = pool.stats();
  const resp: HealthResponse = {
    status: "SERVING",
    active_agents: stats.active,
    max_agents: stats.max,
    uptime_ms: Date.now() - startTime,
  };
  return c.json(resp);
});

app.get("/bridge/active", (c) => {
  return c.json(pool.listActive());
});

// --- Start ---

const port = parseInt(process.env.PORT ?? "7778");
console.log(`[Bridge] Starting on port ${port}`);

export default {
  port,
  fetch: app.fetch,
};

// Graceful shutdown
process.on("SIGTERM", () => {
  console.log("[Bridge] SIGTERM received, shutting down");
  streamer.close();
  process.exit(0);
});
process.on("SIGINT", () => {
  console.log("[Bridge] SIGINT received, shutting down");
  streamer.close();
  process.exit(0);
});
