import { Hono } from "hono";
import type { Context } from "hono";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { EventStreamer } from "./ws/event-stream.js";
import { handleExecute } from "./handlers/execute.js";
import { persistRuntimeSnapshot } from "./handlers/claude-runtime.js";
import type { CommandRuntimeRunner } from "./handlers/command-runtime.js";
import type { QueryRunner } from "./handlers/claude-runtime.js";
import {
  createRuntimeRegistry,
  RuntimeConfigurationError,
  UnknownRuntimeError,
  UnsupportedRuntimeProviderError,
} from "./runtime/registry.js";
import {
  ExecuteRequestSchema,
  CancelRequestSchema,
  ResumeRequestSchema,
  DecomposeTaskRequestSchema,
  DeepReviewRequestSchema,
} from "./schemas.js";
import { handleDecompose, type DecomposeTaskExecutor } from "./handlers/decompose.js";
import { handleClassifyIntent, ClassifyIntentRequestSchema } from "./handlers/classify-intent.js";
import { handleGenerate, GenerateRequestSchema } from "./handlers/generate.js";
import type { HealthResponse } from "./types.js";
import { orchestrateDeepReview } from "./review/orchestrator.js";
import { ToolPluginManager } from "./plugins/tool-plugin-manager.js";
import { PluginRegisterRequestSchema } from "./plugins/schema.js";
import { SessionManager } from "./session/manager.js";
import {
  MissingProviderCredentialsError,
  resolveProviderSelection,
  UnsupportedProviderCapabilityError,
  UnknownProviderError,
} from "./providers/registry.js";
import { MCPClientHub } from "./mcp/client-hub.js";

interface AppDeps {
  pool?: RuntimePoolManager;
  streamer?: Pick<EventStreamer, "connect" | "send" | "close">;
  startTime?: number;
  connectStreamer?: boolean;
  awaitExecution?: boolean;
  queryRunner?: QueryRunner;
  commandRuntimeRunner?: CommandRuntimeRunner;
  sessionManager?: SessionManager;
  now?: () => number;
  executableLookup?: (command: string) => string | null;
  envLookup?: (name: string) => string | undefined;
  decomposeTask?: DecomposeTaskExecutor;
  pluginManager?: ToolPluginManager;
}

function createDefaultPool(): RuntimePoolManager {
  return new RuntimePoolManager(parseInt(process.env.MAX_CONCURRENT_AGENTS ?? "10"));
}

function createDefaultStreamer(): EventStreamer {
  return new EventStreamer(process.env.GO_WS_URL ?? "ws://localhost:7777/ws/bridge");
}

function createDefaultPluginManager(mcpHub?: MCPClientHub, streamer?: Pick<EventStreamer, "send">): ToolPluginManager {
  return new ToolPluginManager({ mcpHub, streamer });
}

function createDefaultSessionManager(): SessionManager {
  return new SessionManager({
    baseDir: process.env.BRIDGE_SESSION_DIR,
  });
}

export function createApp(deps: AppDeps = {}): Hono {
  const app = new Hono();
  const pool = deps.pool ?? createDefaultPool();
  const streamer = deps.streamer ?? createDefaultStreamer();
  const startTime = deps.startTime ?? Date.now();
  const now = deps.now ?? Date.now;
  const mcpHub = new MCPClientHub({
    onToolCallLog: (log) => {
      streamer.send({
        task_id: "__tool__",
        session_id: "",
        timestamp_ms: Date.now(),
        type: "tool.call_log",
        data: log,
      });
    },
  });
  const pluginManager = deps.pluginManager ?? createDefaultPluginManager(mcpHub, streamer);
  const sessionManager = deps.sessionManager ?? createDefaultSessionManager();

  // Wire heartbeat status provider if streamer supports it
  if ("setStatusProvider" in streamer && typeof streamer.setStatusProvider === "function") {
    (streamer as EventStreamer).setStatusProvider({
      getActiveAgentCount: () => pool.stats().active,
      getMCPServerStatuses: () => mcpHub.getAllServerStatuses(),
    });
  }

  if (deps.connectStreamer) {
    streamer.connect();
  }

  function createRegistry() {
    return createRuntimeRegistry({
      queryRunner: deps.queryRunner,
      commandRuntimeRunner: deps.commandRuntimeRunner,
      executableLookup: deps.executableLookup,
      envLookup: deps.envLookup,
      now,
    });
  }

  // ---- Named handlers ----

  async function handleExecuteRoute(c: Context) {
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
        commandRuntimeRunner: deps.commandRuntimeRunner,
        sessionManager,
        executableLookup: deps.executableLookup,
        envLookup: deps.envLookup,
        now,
        pluginManager,
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
      if (err instanceof UnknownRuntimeError || err instanceof UnsupportedRuntimeProviderError) {
        return c.json({ error: message }, 400);
      }
      if (err instanceof RuntimeConfigurationError) {
        return c.json({ error: message }, 503);
      }
      return c.json({ error: message }, 500);
    }
  }

  async function handleDecomposeRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = DecomposeTaskRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }
      const resolvedProvider = resolveProviderSelection("text_generation", parsed.data);
      const result = await handleDecompose(parsed.data, resolvedProvider, deps.decomposeTask);
      return c.json(result, 200);
    } catch (err: unknown) {
      return handleProviderError(c, err);
    }
  }

  async function handleClassifyRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ClassifyIntentRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }
      const resolvedProvider = resolveProviderSelection("text_generation", {});
      const result = await handleClassifyIntent(parsed.data, resolvedProvider);
      return c.json(result, 200);
    } catch (err: unknown) {
      return handleProviderError(c, err);
    }
  }

  async function handleGenerateRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = GenerateRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }
      const resolvedProvider = resolveProviderSelection("text_generation", parsed.data);
      const result = await handleGenerate(parsed.data, resolvedProvider);
      return c.json(result, 200);
    } catch (err: unknown) {
      return handleProviderError(c, err);
    }
  }

  async function handleReviewRoute(c: Context) {
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
  }

  function handleStatusRoute(c: Context) {
    const id = c.req.param("id") ?? "";
    const runtime = pool.get(id);
    if (!runtime) {
      return c.json({ error: "Not found" }, 404);
    }
    return c.json(runtime.toStatus());
  }

  function handleRuntimeCatalogRoute() {
    const catalog = createRegistry().getCatalog();
    return Response.json({
      default_runtime: catalog.defaultRuntime,
      runtimes: catalog.runtimes.map((runtime) => ({
        key: runtime.key,
        label: runtime.label,
        default_provider: runtime.defaultProvider,
        compatible_providers: runtime.compatibleProviders,
        default_model: runtime.defaultModel,
        available: runtime.available,
        diagnostics: runtime.diagnostics,
      })),
    });
  }

  async function handleCancelRoute(c: Context) {
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
      runtime.cancel("cancelled");
      runtime.lastActivity = now();
      return c.json({ success: true });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  }

  async function handlePauseRoute(c: Context) {
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
      if (!runtime || !runtime.request) {
        return c.json({ success: false, error: "Not found" }, 404);
      }

      runtime.lastActivity = now();
      runtime.cancel("paused");
      sessionManager.save(parsed.data.task_id, {
        task_id: runtime.taskId,
        session_id: runtime.sessionId,
        status: "paused",
        turn_number: runtime.turnNumber,
        spent_usd: runtime.spentUsd,
        created_at: runtime.createdAt,
        updated_at: now(),
        request: { ...runtime.request },
      });
      pool.release(parsed.data.task_id);

      return c.json({
        success: true,
        session_id: runtime.sessionId,
        status: "paused",
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  }

  async function handleResumeRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ResumeRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const snapshot = sessionManager.restore(parsed.data.task_id);
      if (!snapshot?.request) {
        return c.json({ error: "Session snapshot not found" }, 404);
      }
      if (pool.get(parsed.data.task_id)) {
        return c.json({ error: "Runtime already active" }, 409);
      }

      const result = await handleExecute(pool, streamer, snapshot.request, {
        awaitCompletion: deps.awaitExecution,
        queryRunner: deps.queryRunner,
        commandRuntimeRunner: deps.commandRuntimeRunner,
        sessionManager,
        executableLookup: deps.executableLookup,
        envLookup: deps.envLookup,
        now,
        pluginManager,
      });

      return c.json({ session_id: result.session_id, resumed: true }, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      if (message.includes("Pool at capacity")) {
        return c.json({ error: message }, 503);
      }
      return c.json({ error: message }, 500);
    }
  }

  function handleHealthRoute(c: Context) {
    const stats = pool.stats();
    const resp: HealthResponse = {
      status: "SERVING",
      active_agents: stats.active,
      max_agents: stats.max,
      uptime_ms: now() - startTime,
    };
    return c.json(resp);
  }

  function handleActiveRoute() {
    return Response.json(pool.listActive());
  }

  function handleToolsListRoute() {
    const allTools = pluginManager.hub.discoverAllTools();
    return Response.json({
      tools: allTools.map(({ pluginId, tool }) => ({
        plugin_id: pluginId,
        name: tool.name,
        description: tool.description,
        input_schema: tool.inputSchema,
      })),
    });
  }

  async function handleToolsInstallRoute(c: Context) {
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
      await pluginManager.enable(record.metadata.id);
      const activated = await pluginManager.activate(record.metadata.id);
      return c.json(activated, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  }

  async function handleToolsUninstallRoute(c: Context) {
    try {
      const body = await c.req.json();
      const pluginId = body?.plugin_id;
      if (!pluginId || typeof pluginId !== "string") {
        return c.json({ error: "plugin_id is required" }, 400);
      }
      const record = await pluginManager.disable(pluginId);
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  }

  async function handleToolsRestartRoute(c: Context) {
    try {
      // Support both /tools/:id/restart (param) and /tools/restart (body plugin_id)
      const id = c.req.param("id") ?? "";
      let pluginId: string = id;
      if (!pluginId) {
        const body = await c.req.json();
        pluginId = body?.plugin_id ?? "";
      }
      if (!pluginId || typeof pluginId !== "string") {
        return c.json({ error: "plugin_id is required" }, 400);
      }
      const record = await pluginManager.restart(pluginId);
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  }

  function handleProviderError(c: Context, err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    if (
      err instanceof UnknownProviderError ||
      err instanceof UnsupportedProviderCapabilityError
    ) {
      return c.json({ error: message }, 400);
    }
    if (err instanceof MissingProviderCredentialsError) {
      return c.json({ error: message }, 503);
    }
    return c.json({ error: message }, 500);
  }

  // ---- Register routes: both /bridge/* (legacy) and /* (PRD) ----

  // Execute
  app.post("/bridge/execute", handleExecuteRoute);
  app.post("/execute", handleExecuteRoute);

  // AI endpoints
  app.post("/bridge/decompose", handleDecomposeRoute);
  app.post("/ai/decompose", handleDecomposeRoute);
  app.post("/bridge/classify-intent", handleClassifyRoute);
  app.post("/ai/classify", handleClassifyRoute);
  app.post("/bridge/generate", handleGenerateRoute);
  app.post("/ai/generate", handleGenerateRoute);

  // Review
  app.post("/bridge/review", handleReviewRoute);

  // Status
  app.get("/bridge/status/:id", handleStatusRoute);
  app.get("/status/:id", handleStatusRoute);
  app.get("/bridge/runtimes", handleRuntimeCatalogRoute);
  app.get("/runtimes", handleRuntimeCatalogRoute);

  // Cancel / Abort
  app.post("/bridge/cancel", handleCancelRoute);
  app.post("/abort", handleCancelRoute);

  // Pause / Resume
  app.post("/bridge/pause", handlePauseRoute);
  app.post("/pause", handlePauseRoute);
  app.post("/bridge/resume", handleResumeRoute);
  app.post("/resume", handleResumeRoute);

  // Health / Active
  app.get("/bridge/health", handleHealthRoute);
  app.get("/health", handleHealthRoute);
  app.get("/bridge/active", handleActiveRoute);
  app.get("/active", handleActiveRoute);

  // Plugin management (internal — no PRD alias needed)
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

  // Tool management
  app.get("/bridge/tools", handleToolsListRoute);
  app.get("/tools", handleToolsListRoute);
  app.post("/bridge/tools/install", handleToolsInstallRoute);
  app.post("/tools/install", handleToolsInstallRoute);
  app.post("/bridge/tools/uninstall", handleToolsUninstallRoute);
  app.post("/tools/uninstall", handleToolsUninstallRoute);
  app.post("/bridge/tools/:id/restart", handleToolsRestartRoute);
  app.post("/tools/restart", handleToolsRestartRoute);

  return app;
}

// ---- Default runtime bootstrap ----

const port = parseInt(process.env.PORT ?? "7778");
let defaultApp: Hono | undefined;
let defaultStreamer: EventStreamer | undefined;
let defaultPool: RuntimePoolManager | undefined;
let defaultPluginManager: ToolPluginManager | undefined;
let defaultSessionManager: SessionManager | undefined;
let signalHandlersRegistered = false;

function getDefaultApp(): Hono {
  if (!defaultApp) {
    console.log(`[Bridge] Starting on port ${port}`);
    defaultStreamer = createDefaultStreamer();
    defaultPool = createDefaultPool();
    defaultSessionManager = createDefaultSessionManager();
    const mcpHub = new MCPClientHub({
      onToolCallLog: (log) => {
        defaultStreamer?.send({
          task_id: "__tool__",
          session_id: "",
          timestamp_ms: Date.now(),
          type: "tool.call_log",
          data: log,
        });
      },
    });
    defaultPluginManager = createDefaultPluginManager(mcpHub, defaultStreamer);
    defaultApp = createApp({
      pool: defaultPool,
      streamer: defaultStreamer,
      connectStreamer: true,
      pluginManager: defaultPluginManager,
      sessionManager: defaultSessionManager,
    });
    if (!signalHandlersRegistered) {
      const gracefulShutdown = async (signal: string) => {
        console.log(`[Bridge] ${signal} received, starting graceful shutdown`);

        // Save snapshots for all active runtimes
        if (defaultPool && defaultStreamer && defaultSessionManager) {
          const runtimes = defaultPool.listRuntimes();
          for (const runtime of runtimes) {
            if (runtime.request && (runtime.status === "running" || runtime.status === "starting")) {
              try {
                persistRuntimeSnapshot(
                  runtime,
                  runtime.request,
                  defaultStreamer,
                  defaultSessionManager,
                  Date.now,
                );
                console.log(`[Bridge] Saved snapshot for task ${runtime.taskId}`);
              } catch (err) {
                console.error(`[Bridge] Failed to save snapshot for task ${runtime.taskId}:`, err);
              }
            }
          }
        }

        // Dispose plugin manager (shut down MCP servers)
        if (defaultPluginManager) {
          try {
            await defaultPluginManager.dispose();
            console.log("[Bridge] Plugin manager disposed");
          } catch (err) {
            console.error("[Bridge] Error disposing plugin manager:", err);
          }
        }

        // Close WS connection
        defaultStreamer?.close();
        console.log("[Bridge] Shutdown complete");
        process.exit(0);
      };

      // Hard timeout for shutdown
      const shutdownWithTimeout = (signal: string) => {
        const timeout = setTimeout(() => {
          console.error("[Bridge] Shutdown timed out after 30s, force exiting");
          process.exit(1);
        }, 30000);
        timeout.unref?.();
        gracefulShutdown(signal);
      };

      process.on("SIGTERM", () => shutdownWithTimeout("SIGTERM"));
      process.on("SIGINT", () => shutdownWithTimeout("SIGINT"));
      signalHandlersRegistered = true;
    }
  }
  return defaultApp;
}

const defaultBridgeApp = {
  port,
  fetch(...args: Parameters<Hono["fetch"]>): ReturnType<Hono["fetch"]> {
    return getDefaultApp().fetch(...args);
  },
};

export default defaultBridgeApp;
