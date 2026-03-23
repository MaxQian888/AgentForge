import { Hono } from "hono";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { EventStreamer } from "./ws/event-stream.js";
import { handleExecute } from "./handlers/execute.js";
import type { QueryRunner } from "./handlers/claude-runtime.js";
import {
  ExecuteRequestSchema,
  CancelRequestSchema,
  DecomposeTaskRequestSchema,
  DeepReviewRequestSchema,
} from "./schemas.js";
import { handleDecompose, type DecomposeTaskExecutor } from "./handlers/decompose.js";
import type { HealthResponse } from "./types.js";
import { orchestrateDeepReview } from "./review/orchestrator.js";
import { ToolPluginManager } from "./plugins/tool-plugin-manager.js";
import { PluginRegisterRequestSchema } from "./plugins/schema.js";
import { SessionManager } from "./session/manager.js";

interface AppDeps {
  pool?: RuntimePoolManager;
  streamer?: Pick<EventStreamer, "connect" | "send" | "close">;
  startTime?: number;
  connectStreamer?: boolean;
  awaitExecution?: boolean;
  queryRunner?: QueryRunner;
  sessionManager?: SessionManager;
  now?: () => number;
  decomposeTask?: DecomposeTaskExecutor;
  pluginManager?: ToolPluginManager;
}

function createDefaultPool(): RuntimePoolManager {
  return new RuntimePoolManager(parseInt(process.env.MAX_CONCURRENT_AGENTS ?? "10"));
}

function createDefaultStreamer(): EventStreamer {
  return new EventStreamer(process.env.GO_WS_URL ?? "ws://localhost:7777/ws/bridge");
}

function createDefaultPluginManager(): ToolPluginManager {
  return new ToolPluginManager();
}

function createDefaultSessionManager(): SessionManager {
  return new SessionManager();
}

export function createApp(deps: AppDeps = {}): Hono {
  const app = new Hono();
  const pool = deps.pool ?? createDefaultPool();
  const streamer = deps.streamer ?? createDefaultStreamer();
  const startTime = deps.startTime ?? Date.now();
  const now = deps.now ?? Date.now;
  const pluginManager = deps.pluginManager ?? createDefaultPluginManager();
  const sessionManager = deps.sessionManager ?? createDefaultSessionManager();

  if (deps.connectStreamer) {
    streamer.connect();
  }

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
      const result = await handleExecute(pool, streamer, parsed.data, {
        awaitCompletion: deps.awaitExecution,
        queryRunner: deps.queryRunner,
        sessionManager,
        now,
      });
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

  app.post("/bridge/decompose", async (c) => {
    try {
      const body = await c.req.json();
      const parsed = DecomposeTaskRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }
      const result = await handleDecompose(parsed.data, deps.decomposeTask);
      return c.json(result, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  });

  app.post("/bridge/review", async (c) => {
    try {
      const body = await c.req.json();
      const parsed = DeepReviewRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const result = await orchestrateDeepReview(parsed.data);
      return c.json(result, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
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
      runtime.lastActivity = now();
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
      uptime_ms: now() - startTime,
    };
    return c.json(resp);
  });

  app.get("/bridge/active", () => {
    return Response.json(pool.listActive());
  });

  app.post("/bridge/plugins/register", async (c) => {
    try {
      const body = await c.req.json();
      const parsed = PluginRegisterRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const record = await pluginManager.register(parsed.data.manifest);
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  });

  app.get("/bridge/plugins", () => {
    return Response.json({ plugins: pluginManager.list() });
  });

  app.post("/bridge/plugins/:id/enable", async (c) => {
    try {
      const record = await pluginManager.enable(c.req.param("id"));
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  });

  app.post("/bridge/plugins/:id/disable", async (c) => {
    try {
      const record = await pluginManager.disable(c.req.param("id"));
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  });

  app.post("/bridge/plugins/:id/activate", async (c) => {
    try {
      const record = await pluginManager.activate(c.req.param("id"));
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  });

  app.get("/bridge/plugins/:id/health", async (c) => {
    try {
      const record = await pluginManager.checkHealth(c.req.param("id"));
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  });

  app.post("/bridge/plugins/:id/restart", async (c) => {
    try {
      const record = await pluginManager.restart(c.req.param("id"));
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  });

  return app;
}

const port = parseInt(process.env.PORT ?? "7778");
let defaultApp: Hono | undefined;
let defaultStreamer: EventStreamer | undefined;
let signalHandlersRegistered = false;

function getDefaultApp(): Hono {
  if (!defaultApp) {
    console.log(`[Bridge] Starting on port ${port}`);
    defaultStreamer = createDefaultStreamer();
    defaultApp = createApp({
      pool: createDefaultPool(),
      streamer: defaultStreamer,
      connectStreamer: true,
    });
    if (!signalHandlersRegistered) {
      process.on("SIGTERM", () => {
        console.log("[Bridge] SIGTERM received, shutting down");
        defaultStreamer?.close();
        process.exit(0);
      });
      process.on("SIGINT", () => {
        console.log("[Bridge] SIGINT received, shutting down");
        defaultStreamer?.close();
        process.exit(0);
      });
      signalHandlersRegistered = true;
    }
  }
  return defaultApp;
}

export default {
  port,
  fetch(...args: Parameters<Hono["fetch"]>): ReturnType<Hono["fetch"]> {
    return getDefaultApp().fetch(...args);
  },
};
