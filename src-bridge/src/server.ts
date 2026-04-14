import { randomUUID } from "node:crypto";
import { Hono } from "hono";
import type { Context } from "hono";
import { AgentRuntime } from "./runtime/agent-runtime.js";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { EventStreamer } from "./ws/event-stream.js";
import { handleExecute } from "./handlers/execute.js";
import { buildRuntimeSnapshot, persistRuntimeSnapshot } from "./handlers/claude-runtime.js";
import type { CommandRuntimeRunner } from "./handlers/command-runtime.js";
import type { CodexAuthStatusProvider, CodexRuntimeRunner } from "./handlers/codex-runtime.js";
import type { OpenCodeEventRunner } from "./handlers/opencode-runtime.js";
import type { QueryRunner } from "./handlers/claude-runtime.js";
import {
  type AgentRuntimeRegistry,
  createRuntimeRegistry,
  ExecuteRequestValidationError,
  RuntimeConfigurationError,
  UnknownRuntimeError,
  UnsupportedRuntimeProviderError,
} from "./runtime/registry.js";
import {
  CommandRequestSchema,
  ExecuteRequestSchema,
  CancelRequestSchema,
  ForkRequestSchema,
  InterruptRequestSchema,
  ModelSwitchRequestSchema,
  PermissionResponseSchema,
  RevertRequestSchema,
  RollbackRequestSchema,
  ResumeRequestSchema,
  ShellRequestSchema,
  ThinkingBudgetRequestSchema,
  UnrevertRequestSchema,
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
import {
  createOpenCodeTransport,
  type OpenCodeTransport,
} from "./opencode/transport.js";
import { OpenCodePendingInteractionStore } from "./opencode/pending-interactions.js";
import { BunSchedulerAdapter } from "./scheduler/bun-cron-adapter.js";
import { HookCallbackManager } from "./runtime/hook-callback-manager.js";
import { UnsupportedOperationError } from "./runtime/errors.js";
import { getRuntimeProfile } from "./runtime/backend-profiles.js";

interface AppDeps {
  pool?: RuntimePoolManager;
  streamer?: Pick<EventStreamer, "connect" | "send" | "close">;
  startTime?: number;
  connectStreamer?: boolean;
  awaitExecution?: boolean;
  queryRunner?: QueryRunner;
  commandRuntimeRunner?: CommandRuntimeRunner;
  codexRuntimeRunner?: CodexRuntimeRunner;
  sessionManager?: SessionManager;
  now?: () => number;
  executableLookup?: (command: string) => string | null;
  envLookup?: (name: string) => string | undefined;
  codexAuthStatusProvider?: CodexAuthStatusProvider;
  opencodeTransport?: OpenCodeTransport;
  opencodeEventRunner?: OpenCodeEventRunner;
  decomposeTask?: DecomposeTaskExecutor;
  pluginManager?: ToolPluginManager;
  schedulerAdapter?: Pick<BunSchedulerAdapter, "start" | "stop">;
  runtimeRegistry?: AgentRuntimeRegistry;
  hookCallbackManager?: HookCallbackManager;
  opencodePendingInteractions?: {
    createPermissionRequest?: (input: {
      sessionId: string;
      permissionId: string;
      toolName?: string;
      context?: unknown;
      ttlMs?: number;
    }) => { requestId: string };
    getPermissionRequest?: (
      requestId: string,
    ) => { sessionId: string; permissionId: string } | null;
    consumePermissionRequest?: (requestId: string) => boolean;
    resolvePermissionResponse?: (
      requestId: string,
      payload: { decision: "allow" | "deny"; reason?: string },
    ) =>
      | boolean
      | {
          sessionId: string;
          permissionId: string;
          allow: boolean;
          reason?: string;
        }
      | null;
    createProviderAuthRequest?: (input: { provider: string; ttlMs?: number }) => {
      requestId: string;
    };
    consumeProviderAuthRequest?: (
      requestId: string,
    ) => { provider: string } | null;
  };
}

type BridgeRouteMethod = "get" | "post";

interface BridgeHttpRouteGroup {
  method: BridgeRouteMethod;
  canonicalPath: string;
  compatibilityAliases: readonly string[];
}

export const BRIDGE_HTTP_ROUTE_GROUPS = {
  execute: {
    method: "post",
    canonicalPath: "/bridge/execute",
    compatibilityAliases: ["/execute"],
  },
  decompose: {
    method: "post",
    canonicalPath: "/bridge/decompose",
    compatibilityAliases: ["/ai/decompose"],
  },
  classifyIntent: {
    method: "post",
    canonicalPath: "/bridge/classify-intent",
    compatibilityAliases: ["/ai/classify"],
  },
  generate: {
    method: "post",
    canonicalPath: "/bridge/generate",
    compatibilityAliases: ["/ai/generate"],
  },
  review: {
    method: "post",
    canonicalPath: "/bridge/review",
    compatibilityAliases: [],
  },
  status: {
    method: "get",
    canonicalPath: "/bridge/status/:id",
    compatibilityAliases: ["/status/:id"],
  },
  runtimes: {
    method: "get",
    canonicalPath: "/bridge/runtimes",
    compatibilityAliases: ["/runtimes"],
  },
  fork: {
    method: "post",
    canonicalPath: "/bridge/fork",
    compatibilityAliases: [],
  },
  rollback: {
    method: "post",
    canonicalPath: "/bridge/rollback",
    compatibilityAliases: [],
  },
  revert: {
    method: "post",
    canonicalPath: "/bridge/revert",
    compatibilityAliases: [],
  },
  unrevert: {
    method: "post",
    canonicalPath: "/bridge/unrevert",
    compatibilityAliases: [],
  },
  diff: {
    method: "get",
    canonicalPath: "/bridge/diff/:task_id",
    compatibilityAliases: [],
  },
  messages: {
    method: "get",
    canonicalPath: "/bridge/messages/:task_id",
    compatibilityAliases: [],
  },
  shell: {
    method: "post",
    canonicalPath: "/bridge/shell",
    compatibilityAliases: [],
  },
  thinking: {
    method: "post",
    canonicalPath: "/bridge/thinking",
    compatibilityAliases: [],
  },
  command: {
    method: "post",
    canonicalPath: "/bridge/command",
    compatibilityAliases: [],
  },
  interrupt: {
    method: "post",
    canonicalPath: "/bridge/interrupt",
    compatibilityAliases: [],
  },
  model: {
    method: "post",
    canonicalPath: "/bridge/model",
    compatibilityAliases: [],
  },
  mcpStatus: {
    method: "get",
    canonicalPath: "/bridge/mcp-status/:task_id",
    compatibilityAliases: [],
  },
  permissionResponse: {
    method: "post",
    canonicalPath: "/bridge/permission-response/:request_id",
    compatibilityAliases: [],
  },
  cancel: {
    method: "post",
    canonicalPath: "/bridge/cancel",
    compatibilityAliases: ["/abort"],
  },
  pause: {
    method: "post",
    canonicalPath: "/bridge/pause",
    compatibilityAliases: ["/pause"],
  },
  resume: {
    method: "post",
    canonicalPath: "/bridge/resume",
    compatibilityAliases: ["/resume"],
  },
  health: {
    method: "get",
    canonicalPath: "/bridge/health",
    compatibilityAliases: ["/health"],
  },
  active: {
    method: "get",
    canonicalPath: "/bridge/active",
    compatibilityAliases: ["/active"],
  },
  pool: {
    method: "get",
    canonicalPath: "/bridge/pool",
    compatibilityAliases: ["/pool"],
  },
  toolsList: {
    method: "get",
    canonicalPath: "/bridge/tools",
    compatibilityAliases: ["/tools"],
  },
  toolsInstall: {
    method: "post",
    canonicalPath: "/bridge/tools/install",
    compatibilityAliases: ["/tools/install"],
  },
  toolsUninstall: {
    method: "post",
    canonicalPath: "/bridge/tools/uninstall",
    compatibilityAliases: ["/tools/uninstall"],
  },
  toolsRestart: {
    method: "post",
    canonicalPath: "/bridge/tools/:id/restart",
    compatibilityAliases: ["/tools/restart"],
  },
} as const satisfies Record<string, BridgeHttpRouteGroup>;

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

function createDefaultSchedulerAdapter(): BunSchedulerAdapter {
  return new BunSchedulerAdapter({
    goApiUrl: process.env.GO_API_URL ?? "http://localhost:7777",
  });
}

function registerBridgeRouteGroup(
  app: Hono,
  group: BridgeHttpRouteGroup,
  handler: (c: Context) => Response | Promise<Response>,
) {
  const register = group.method === "get" ? app.get.bind(app) : app.post.bind(app);
  register(group.canonicalPath, handler);
  for (const alias of group.compatibilityAliases) {
    register(alias, handler);
  }
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
  const hookCallbackManager = deps.hookCallbackManager ?? new HookCallbackManager();
  const opencodeTransport =
    deps.opencodeTransport ??
    createOpenCodeTransport({
      envLookup: deps.envLookup,
    });
  const opencodePendingInteractions =
    deps.opencodePendingInteractions ?? new OpenCodePendingInteractionStore();
  const opencodeRuntimePendingInteractions =
    opencodePendingInteractions.createPermissionRequest
      ? {
          createPermissionRequest:
            opencodePendingInteractions.createPermissionRequest.bind(
              opencodePendingInteractions,
            ),
        }
      : undefined;

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
    if (deps.runtimeRegistry) {
      return deps.runtimeRegistry;
    }
    return createRuntimeRegistry({
      queryRunner: deps.queryRunner,
      commandRuntimeRunner: deps.commandRuntimeRunner,
      codexRuntimeRunner: deps.codexRuntimeRunner,
      executableLookup: deps.executableLookup,
      envLookup: deps.envLookup,
      codexAuthStatusProvider: deps.codexAuthStatusProvider,
      opencodeTransport,
      opencodeEventRunner: deps.opencodeEventRunner,
      now,
      opencodePendingInteractions: opencodeRuntimePendingInteractions,
    });
  }

  function hydrateRuntimeFromSnapshot(snapshot: {
    task_id: string;
    session_id: string;
    status: string;
    turn_number: number;
    spent_usd: number;
    updated_at: number;
    request?: Record<string, unknown>;
    continuity?: Record<string, unknown>;
  }): AgentRuntime {
    const runtime = new AgentRuntime(snapshot.task_id, snapshot.session_id);
    if (snapshot.request) {
      runtime.bindRequest(snapshot.request as never);
    }
    runtime.continuity = snapshot.continuity as never;
    runtime.status = snapshot.status as never;
    runtime.turnNumber = snapshot.turn_number;
    runtime.spentUsd = snapshot.spent_usd;
    runtime.lastActivity = snapshot.updated_at;
    return runtime;
  }

function resolveRuntimeForBridgeControl(taskId: string):
  | { runtime: AgentRuntime }
  | { error: { status: 404 | 409; body: Record<string, unknown> } } {
    const activeRuntime = pool.get(taskId);
    if (activeRuntime) {
      return { runtime: activeRuntime };
    }

    const snapshot = sessionManager.restore(taskId);
    if (!snapshot?.request) {
      return {
        error: {
          status: 404,
          body: { error: "task not found" },
        },
      };
    }

    const snapshotRuntime =
      snapshot.request.runtime ?? inferRuntimeFromProvider(snapshot.request.provider);
    if (snapshotRuntime !== "opencode") {
      return {
        error: {
          status: 404,
          body: { error: "task not found" },
        },
      };
    }

    if (
      snapshot.continuity?.runtime !== "opencode" ||
      !snapshot.continuity.upstream_session_id
    ) {
      return {
        error: {
          status: 409,
          body: {
            error: `OpenCode continuity state is not available for task ${taskId}`,
            code: snapshot.continuity?.blocking_reason ?? "missing_continuity_state",
            runtime: "opencode",
          },
        },
      };
    }

    return {
      runtime: hydrateRuntimeFromSnapshot(snapshot as never),
    };
  }

  function resolveRuntimeForRollbackControl(taskId: string):
    | { runtime: AgentRuntime }
    | { error: { status: 404 | 409; body: Record<string, unknown> } } {
      const activeRuntime = pool.get(taskId);
      if (activeRuntime) {
        return { runtime: activeRuntime };
      }

      const snapshot = sessionManager.restore(taskId);
      if (!snapshot?.request) {
        return {
          error: {
            status: 404,
            body: { error: "task not found" },
          },
        };
      }

      const snapshotRuntime =
        snapshot.request.runtime ?? inferRuntimeFromProvider(snapshot.request.provider);
      if (!snapshotRuntime) {
        return {
          error: {
            status: 404,
            body: { error: "task not found" },
          },
        };
      }

      if (snapshot.continuity?.runtime !== snapshotRuntime) {
        return {
          error: {
            status: 409,
            body: {
              error: `${runtimeDisplayLabel(snapshotRuntime)} continuity state is not available for task ${taskId}`,
              code: snapshot.continuity?.blocking_reason ?? "missing_continuity_state",
              runtime: snapshotRuntime,
            },
          },
        };
      }

      return {
        runtime: hydrateRuntimeFromSnapshot(snapshot as never),
      };
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
        codexRuntimeRunner: deps.codexRuntimeRunner,
        sessionManager,
        executableLookup: deps.executableLookup,
        envLookup: deps.envLookup,
        now,
        codexAuthStatusProvider: deps.codexAuthStatusProvider,
        opencodeTransport,
        opencodeEventRunner: deps.opencodeEventRunner,
        runtimeRegistry: deps.runtimeRegistry,
        hookCallbackManager,
        pluginManager,
        opencodePendingInteractions: opencodeRuntimePendingInteractions,
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
      if (err instanceof ExecuteRequestValidationError) {
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

  async function handleRuntimeCatalogRoute() {
    const catalog = await createRegistry().getCatalog();
      return Response.json({
      default_runtime: catalog.defaultRuntime,
      runtimes: catalog.runtimes.map((runtime) => ({
        key: runtime.key,
        label: runtime.label,
        default_provider: runtime.defaultProvider,
        compatible_providers: runtime.compatibleProviders,
        default_model: runtime.defaultModel,
        model_options: runtime.modelOptions,
        available: runtime.available,
        diagnostics: runtime.diagnostics,
        supported_features: runtime.supportedFeatures,
        interaction_capabilities: serializeInteractionCapabilities(
          runtime.interactionCapabilities,
        ),
        launch_contract: runtime.launchContract
          ? {
              prompt_transport: runtime.launchContract.promptTransport,
              output_mode: runtime.launchContract.outputMode,
              supported_output_modes: runtime.launchContract.supportedOutputModes,
              supported_approval_modes: runtime.launchContract.supportedApprovalModes,
              additional_directories: runtime.launchContract.additionalDirectories,
              env_overrides: runtime.launchContract.envOverrides,
            }
          : undefined,
        lifecycle: runtime.lifecycle
          ? {
              stage: runtime.lifecycle.stage,
              sunset_at: runtime.lifecycle.sunsetAt,
              replacement_runtime: runtime.lifecycle.replacementRuntime,
              message: runtime.lifecycle.message,
            }
          : undefined,
        agents: runtime.agents,
        skills: runtime.skills,
        providers: runtime.providers?.map((provider) => ({
          provider: provider.provider,
          connected: provider.connected,
          default_model: provider.defaultModel,
          model_options: provider.modelOptions,
          auth_required: provider.authRequired,
          auth_methods: provider.authMethods,
        })),
      })),
    });
  }

  async function handleForkRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ForkRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const runtime = pool.get(parsed.data.task_id);
      if (!runtime?.request) {
        return c.json({ error: "task not found" }, 404);
      }

      const forked = await createRegistry().fork(runtime, {
        message_id: parsed.data.message_id,
      });
      const newTaskId = randomUUID();
      const newSessionId = randomUUID();
      sessionManager.save(newTaskId, {
        task_id: newTaskId,
        session_id: newSessionId,
        status: "paused",
        turn_number: runtime.turnNumber,
        spent_usd: runtime.spentUsd,
        created_at: now(),
        updated_at: now(),
        request: {
          ...runtime.request,
          task_id: newTaskId,
          session_id: newSessionId,
        },
        continuity: forked.continuity,
      });

      return c.json(
        {
          new_task_id: newTaskId,
          continuity: forked.continuity,
        },
        200,
      );
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleRollbackRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = RollbackRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const resolvedRuntime = resolveRuntimeForRollbackControl(parsed.data.task_id);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      await createRegistry().rollback(resolvedRuntime.runtime, {
        checkpoint_id: parsed.data.checkpoint_id,
        turns: parsed.data.turns,
      });
      resolvedRuntime.runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleRevertRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = RevertRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const resolvedRuntime = resolveRuntimeForBridgeControl(parsed.data.task_id);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      await createRegistry().revert(resolvedRuntime.runtime, {
        action: "revert",
        message_id: parsed.data.message_id,
      });
      resolvedRuntime.runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleUnrevertRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = UnrevertRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const resolvedRuntime = resolveRuntimeForBridgeControl(parsed.data.task_id);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      await createRegistry().revert(resolvedRuntime.runtime, {
        action: "unrevert",
      });
      resolvedRuntime.runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleDiffRoute(c: Context) {
    try {
      const taskId = c.req.param("task_id") ?? "";
      const resolvedRuntime = resolveRuntimeForBridgeControl(taskId);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      const diff = await createRegistry().getDiff(resolvedRuntime.runtime);
      return c.json(diff, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleMessagesRoute(c: Context) {
    try {
      const taskId = c.req.param("task_id") ?? "";
      const resolvedRuntime = resolveRuntimeForBridgeControl(taskId);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      const messages = await createRegistry().getMessages(resolvedRuntime.runtime);
      return c.json(messages, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleShellRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ShellRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const resolvedRuntime = resolveRuntimeForBridgeControl(parsed.data.task_id);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      const result = await createRegistry().executeShell(resolvedRuntime.runtime, {
        command: parsed.data.command,
        agent: parsed.data.agent,
        model: parsed.data.model,
      });
      resolvedRuntime.runtime.lastActivity = now();
      return c.json(normalizeShellRouteResponse(resolvedRuntime.runtime, result), 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleThinkingRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ThinkingBudgetRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const runtime = pool.get(parsed.data.task_id);
      if (!runtime) {
        return c.json({ error: "task not found" }, 404);
      }

      await createRegistry().setThinkingBudget(runtime, {
        max_thinking_tokens: parsed.data.max_thinking_tokens ?? null,
      });
      runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleCommandRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = CommandRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const resolvedRuntime = resolveRuntimeForBridgeControl(parsed.data.task_id);
      if ("error" in resolvedRuntime) {
        return c.json(resolvedRuntime.error.body, resolvedRuntime.error.status);
      }

      const result = await createRegistry().executeCommand(resolvedRuntime.runtime, {
        command: parsed.data.command,
        arguments: parsed.data.arguments,
      });
      resolvedRuntime.runtime.lastActivity = now();
      return c.json(result ?? { success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleInterruptRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = InterruptRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const runtime = pool.get(parsed.data.task_id);
      if (!runtime) {
        return c.json({ error: "task not found" }, 404);
      }

      await createRegistry().interrupt(runtime);
      runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleModelRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = ModelSwitchRequestSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const runtime = pool.get(parsed.data.task_id);
      if (!runtime?.request) {
        return c.json({ error: "task not found" }, 404);
      }

      await createRegistry().setModel(runtime, {
        model: parsed.data.model,
      });
      runtime.request = {
        ...runtime.request,
        model: parsed.data.model,
      };
      runtime.lastActivity = now();
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handleMcpStatusRoute(c: Context) {
    try {
      const taskId = c.req.param("task_id") ?? "";
      const runtime = pool.get(taskId);
      if (!runtime) {
        return c.json({ error: "task not found" }, 404);
      }

      const status = await createRegistry().getMcpServerStatus(runtime);
      runtime.lastActivity = now();
      return c.json(status, 200);
    } catch (err: unknown) {
      return handleOperationError(c, err);
    }
  }

  async function handlePermissionResponseRoute(c: Context) {
    try {
      const body = await c.req.json();
      const parsed = PermissionResponseSchema.safeParse(body);
      if (!parsed.success) {
        return c.json(
          { error: "Validation failed", details: parsed.error.flatten() },
          400,
        );
      }

      const requestId = c.req.param("request_id") ?? "";
      const pendingPermission =
        opencodePendingInteractions.getPermissionRequest?.(requestId);
      if (pendingPermission) {
        await opencodeTransport.respondToPermission(
          pendingPermission.sessionId,
          pendingPermission.permissionId,
          parsed.data.decision === "allow",
        );
        opencodePendingInteractions.consumePermissionRequest?.(requestId);
        return c.json({ success: true }, 200);
      }
      const opencodeResolved =
        opencodePendingInteractions.resolvePermissionResponse?.(requestId, parsed.data);
      if (opencodeResolved) {
        if (
          typeof opencodeResolved === "object" &&
          "sessionId" in opencodeResolved &&
          "permissionId" in opencodeResolved
        ) {
          await opencodeTransport.respondToPermission(
            opencodeResolved.sessionId,
            opencodeResolved.permissionId,
            opencodeResolved.allow,
          );
        }
        return c.json({ success: true }, 200);
      }
      const resolved = hookCallbackManager.resolve(requestId, parsed.data);
      if (!resolved) {
        return c.json({ error: "pending permission request not found" }, 404);
      }
      return c.json({ success: true }, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
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
      if (
        runtime.request?.runtime === "opencode" &&
        runtime.continuity?.runtime === "opencode" &&
        runtime.continuity.upstream_session_id &&
        opencodeTransport
      ) {
        await opencodeTransport.abortSession(runtime.continuity.upstream_session_id);
        runtime.continuity = {
          ...runtime.continuity,
          resume_ready: false,
          captured_at: now(),
          blocking_reason: "continuity_not_supported",
        };
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
      if (
        runtime.request.runtime === "opencode" &&
        runtime.continuity?.runtime === "opencode" &&
        runtime.continuity.upstream_session_id &&
        opencodeTransport
      ) {
        await opencodeTransport.abortSession(runtime.continuity.upstream_session_id);
        runtime.continuity = {
          ...runtime.continuity,
          resume_ready: true,
          captured_at: now(),
        };
      } else if (
        runtime.request.runtime === "cursor" ||
        runtime.request.runtime === "gemini" ||
        runtime.request.runtime === "qoder" ||
        runtime.request.runtime === "iflow"
      ) {
        runtime.continuity = {
          runtime: runtime.request.runtime,
          resume_ready: false,
          captured_at: now(),
          blocking_reason: "continuity_not_supported",
        };
      }
      runtime.cancel("paused");
      sessionManager.save(parsed.data.task_id, buildRuntimeSnapshot(runtime, runtime.request, now));
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
      const resumeBlock = getResumeBlock(snapshot);
      if (resumeBlock) {
        return c.json(
          {
            error: `${resumeBlock.runtimeLabel} continuity state is not resumable for task ${parsed.data.task_id}`,
            code: resumeBlock.code,
          },
          409,
        );
      }
      const contextMismatch = getResumeContextMismatch(snapshot.request, parsed.data);
      if (contextMismatch) {
        return c.json(
          {
            error: `Resume request context mismatch for ${contextMismatch.field}: expected ${contextMismatch.expected}, got ${contextMismatch.actual}`,
            code: "resume_context_mismatch",
            field: contextMismatch.field,
          },
          409,
        );
      }

      const result = await handleExecute(pool, streamer, snapshot.request, {
        awaitCompletion: deps.awaitExecution,
        continuity: snapshot.continuity,
        queryRunner: deps.queryRunner,
        commandRuntimeRunner: deps.commandRuntimeRunner,
        codexRuntimeRunner: deps.codexRuntimeRunner,
        sessionManager,
        executableLookup: deps.executableLookup,
        envLookup: deps.envLookup,
        now,
        codexAuthStatusProvider: deps.codexAuthStatusProvider,
        opencodeTransport,
        opencodeEventRunner: deps.opencodeEventRunner,
        hookCallbackManager,
        pluginManager,
        opencodePendingInteractions: opencodeRuntimePendingInteractions,
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

  function handlePoolRoute() {
    return Response.json(pool.stats());
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

  async function handlePluginMcpRefreshRoute(c: Context) {
    try {
      const pluginId = c.req.param("id");
      if (!pluginId) {
        return c.json({ error: "plugin id is required" }, 400);
      }
      const record = await pluginManager.refreshCapabilitySurface(pluginId);
      return c.json(record, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  }

  async function handlePluginMcpToolCallRoute(c: Context) {
    try {
      const pluginId = c.req.param("id");
      if (!pluginId) {
        return c.json({ error: "plugin id is required" }, 400);
      }
      const body = await c.req.json();
      const toolName = body?.tool_name;
      const args = body?.arguments ?? {};
      if (!toolName || typeof toolName !== "string") {
        return c.json({ error: "tool_name is required" }, 400);
      }
      const result = await pluginManager.invokeTool(pluginId, toolName, args);
      return c.json({
        plugin_id: pluginId,
        operation: "call_tool",
        result,
      }, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  }

  async function handlePluginMcpResourceReadRoute(c: Context) {
    try {
      const pluginId = c.req.param("id");
      if (!pluginId) {
        return c.json({ error: "plugin id is required" }, 400);
      }
      const body = await c.req.json();
      const uri = body?.uri;
      if (!uri || typeof uri !== "string") {
        return c.json({ error: "uri is required" }, 400);
      }
      const result = await pluginManager.readResource(pluginId, uri);
      return c.json({
        plugin_id: pluginId,
        operation: "read_resource",
        result,
      }, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 400);
    }
  }

  async function handlePluginMcpPromptGetRoute(c: Context) {
    try {
      const pluginId = c.req.param("id");
      if (!pluginId) {
        return c.json({ error: "plugin id is required" }, 400);
      }
      const body = await c.req.json();
      const name = body?.name;
      const args = body?.arguments;
      if (!name || typeof name !== "string") {
        return c.json({ error: "name is required" }, 400);
      }
      const result = await pluginManager.getPrompt(pluginId, name, args);
      return c.json({
        plugin_id: pluginId,
        operation: "get_prompt",
        result,
      }, 200);
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

  // ---- Register live TS Bridge contract ----
  // New callers, tests, and docs MUST use the canonical /bridge/* paths below.
  // Compatibility aliases are kept only to avoid breaking legacy callers.
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.execute, handleExecuteRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.decompose, handleDecomposeRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.classifyIntent, handleClassifyRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.generate, handleGenerateRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.review, handleReviewRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.status, handleStatusRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.runtimes, handleRuntimeCatalogRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.fork, handleForkRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.rollback, handleRollbackRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.revert, handleRevertRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.unrevert, handleUnrevertRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.diff, handleDiffRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.messages, handleMessagesRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.shell, handleShellRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.thinking, handleThinkingRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.command, handleCommandRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.interrupt, handleInterruptRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.model, handleModelRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.mcpStatus, handleMcpStatusRoute);
  registerBridgeRouteGroup(
    app,
    BRIDGE_HTTP_ROUTE_GROUPS.permissionResponse,
    handlePermissionResponseRoute,
  );
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.cancel, handleCancelRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.pause, handlePauseRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.resume, handleResumeRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.health, handleHealthRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.active, handleActiveRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.pool, handlePoolRoute);

  app.post("/bridge/opencode/provider-auth/:provider/start", async (c) => {
    try {
      const provider = c.req.param("provider") ?? "";
      if (!provider) {
        return c.json({ error: "provider is required" }, 400);
      }
      const body = await c.req.json().catch(() => ({}));
      const auth = await opencodeTransport.startProviderOAuth(
        provider,
        body && typeof body === "object" ? (body as Record<string, unknown>) : {},
      );
      const pending = opencodePendingInteractions.createProviderAuthRequest?.({ provider });
      if (!pending) {
        return c.json({ error: "OpenCode provider auth storage is unavailable" }, 500);
      }
      return c.json(
        {
          request_id: pending.requestId,
          provider,
          auth,
        },
        200,
      );
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  });

  app.post("/bridge/opencode/provider-auth/:request_id/complete", async (c) => {
    try {
      const requestId = c.req.param("request_id") ?? "";
      const pending = opencodePendingInteractions.consumeProviderAuthRequest?.(requestId);
      if (!pending) {
        return c.json({ error: "pending provider auth request not found" }, 404);
      }
      const body = await c.req.json().catch(() => ({}));
      const result = await opencodeTransport.completeProviderOAuth(
        pending.provider,
        body && typeof body === "object" ? (body as Record<string, unknown>) : {},
      );
      return c.json(result, 200);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      return c.json({ error: message }, 500);
    }
  });

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

  app.post("/bridge/plugins/:id/mcp/refresh", handlePluginMcpRefreshRoute);
  app.post("/bridge/plugins/:id/mcp/tools/call", handlePluginMcpToolCallRoute);
  app.post("/bridge/plugins/:id/mcp/resources/read", handlePluginMcpResourceReadRoute);
  app.post("/bridge/plugins/:id/mcp/prompts/get", handlePluginMcpPromptGetRoute);

  // Tool management keeps the same canonical/compatibility split as the rest of the contract.
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.toolsList, handleToolsListRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.toolsInstall, handleToolsInstallRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.toolsUninstall, handleToolsUninstallRoute);
  registerBridgeRouteGroup(app, BRIDGE_HTTP_ROUTE_GROUPS.toolsRestart, handleToolsRestartRoute);

  return app;
}

function handleOperationError(c: Context, err: unknown) {
  const message = err instanceof Error ? err.message : String(err);
  if (err instanceof UnsupportedOperationError) {
    return c.json(
      {
        error: message,
        operation: err.operation,
        runtime: err.runtime,
        support_state: err.supportState,
        reason_code: err.reasonCode,
      },
      err.supportState === "degraded" ? 409 : 501,
    );
  }
  return c.json({ error: message }, 500);
}

function normalizeShellRouteResponse(runtime: AgentRuntime, result: unknown) {
  if (isObjectRecord(result) && typeof result.success === "boolean") {
    return result;
  }

  const normalized: Record<string, unknown> = {
    success: inferShellSuccess(result),
    task_id: runtime.taskId,
    session_id: runtime.sessionId,
  };

  if (typeof result === "string") {
    normalized.output = result;
    return normalized;
  }

  if (result == null) {
    return normalized;
  }

  if (isObjectRecord(result)) {
    if (typeof result.output === "string") {
      normalized.output = result.output;
      return normalized;
    }
    if (typeof result.error === "string") {
      normalized.error = result.error;
      return normalized;
    }
    if (typeof result.message === "string" && normalized.success === false) {
      normalized.error = result.message;
      return normalized;
    }
  }

  const serialized = safeJSONStringify(result);
  if (serialized) {
    if (normalized.success === false) {
      normalized.error = serialized;
    } else {
      normalized.output = serialized;
    }
  }
  return normalized;
}

function inferShellSuccess(result: unknown): boolean {
  if (isObjectRecord(result)) {
    if (typeof result.success === "boolean") {
      return result.success;
    }
    if (typeof result.ok === "boolean") {
      return result.ok;
    }
    if (typeof result.error === "string" && result.error.trim().length > 0) {
      return false;
    }
  }
  return true;
}

function safeJSONStringify(value: unknown): string {
  try {
    return JSON.stringify(value) ?? "";
  } catch {
    return String(value);
  }
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function serializeInteractionCapabilities(
  capabilities:
    | {
        inputs: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>;
        lifecycle: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>;
        approval: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>;
        mcp: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>;
        diagnostics: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>;
      }
    | undefined,
) {
  if (!capabilities) {
    return undefined;
  }

  const serializeGroup = (
    group: Record<string, { state: string; reasonCode?: string; message?: string; requiresRequestFields?: string[] }>,
  ) =>
    Object.fromEntries(
      Object.entries(group).map(([key, value]) => [
        key,
        {
          state: value.state,
          reason_code: value.reasonCode,
          message: value.message,
          requires_request_fields: value.requiresRequestFields,
        },
      ]),
    );

  return {
    inputs: serializeGroup(capabilities.inputs),
    lifecycle: serializeGroup(capabilities.lifecycle),
    approval: serializeGroup(capabilities.approval),
    mcp: serializeGroup(capabilities.mcp),
    diagnostics: serializeGroup(capabilities.diagnostics),
  };
}

function getResumeBlock(snapshot: {
  request?: { runtime?: string; provider?: string };
  continuity?: {
    runtime: "claude_code" | "codex" | "opencode" | "cursor" | "gemini" | "qoder" | "iflow";
    resume_ready: boolean;
    blocking_reason?: string;
    thread_id?: string;
  };
}): { runtimeLabel: string; code: string } | null {
  if (snapshot.continuity) {
    if (
      snapshot.continuity.resume_ready &&
      (snapshot.continuity.runtime !== "codex" || Boolean(snapshot.continuity.thread_id))
    ) {
      return null;
    }
    return {
      runtimeLabel: runtimeDisplayLabel(snapshot.continuity.runtime),
      code: snapshot.continuity.blocking_reason ?? "missing_continuity_state",
    };
  }

  const runtime = snapshot.request?.runtime ?? inferRuntimeFromProvider(snapshot.request?.provider);
  if (!runtime) {
    return null;
  }

  return {
    runtimeLabel: runtimeDisplayLabel(runtime),
    code: "missing_continuity_state",
  };
}

function getResumeContextMismatch(
  snapshotRequest:
    | {
        session_id?: string;
        runtime?: string;
        provider?: string;
        model?: string;
        team_id?: string;
        team_role?: string;
      }
    | undefined,
  resumeRequest: {
    session_id?: string;
    runtime?: string;
    provider?: string;
    model?: string;
    team_id?: string;
    team_role?: string;
  },
): { field: string; expected: string; actual: string } | null {
  if (!snapshotRequest) {
    return null;
  }
  const comparisons: Array<[string, string | undefined, string | undefined]> = [
    ["session_id", snapshotRequest.session_id, resumeRequest.session_id],
    ["runtime", snapshotRequest.runtime, resumeRequest.runtime],
    ["provider", snapshotRequest.provider, resumeRequest.provider],
    ["model", snapshotRequest.model, resumeRequest.model],
    ["team_id", snapshotRequest.team_id, resumeRequest.team_id],
    ["team_role", snapshotRequest.team_role, resumeRequest.team_role],
  ];
  for (const [field, expected, actual] of comparisons) {
    if (typeof actual !== "string" || actual.trim() === "") {
      continue;
    }
    const normalizedExpected = typeof expected === "string" ? expected.trim() : "";
    if (normalizedExpected === "") {
      continue;
    }
    if (normalizedExpected !== actual.trim()) {
      return { field, expected: normalizedExpected, actual: actual.trim() };
    }
  }
  return null;
}

function inferRuntimeFromProvider(
  provider: string | undefined,
): "claude_code" | "codex" | "opencode" | "cursor" | "gemini" | "qoder" | "iflow" | null {
  switch (provider) {
    case "anthropic":
      return "claude_code";
    case "openai":
    case "codex":
      return "codex";
    case "opencode":
      return "opencode";
    case "cursor":
      return "cursor";
    case "google":
    case "vertex":
      return "gemini";
    case "qoder":
      return "qoder";
    case "iflow":
      return "iflow";
    default:
      return null;
  }
}

function runtimeDisplayLabel(runtime: string): string {
  try {
    return getRuntimeProfile(
      runtime as "claude_code" | "codex" | "opencode" | "cursor" | "gemini" | "qoder" | "iflow",
    ).label;
  } catch {
    return runtime;
  }
}

// ---- Default runtime bootstrap ----

const port = parseInt(process.env.PORT ?? "7778");
let defaultApp: Hono | undefined;
let defaultStreamer: EventStreamer | undefined;
let defaultPool: RuntimePoolManager | undefined;
let defaultPluginManager: ToolPluginManager | undefined;
let defaultSessionManager: SessionManager | undefined;
let defaultSchedulerAdapter: BunSchedulerAdapter | undefined;
let signalHandlersRegistered = false;

function getDefaultApp(): Hono {
  if (!defaultApp) {
    console.log(`[Bridge] Starting on port ${port}`);
    defaultStreamer = createDefaultStreamer();
    defaultPool = createDefaultPool();
    defaultSessionManager = createDefaultSessionManager();
    defaultSchedulerAdapter = createDefaultSchedulerAdapter();
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
      schedulerAdapter: defaultSchedulerAdapter,
    });
    void defaultSchedulerAdapter.start().catch((error) => {
      console.warn("[Bridge] Failed to start Bun scheduler adapter:", error);
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

        if (defaultSchedulerAdapter) {
          try {
            await defaultSchedulerAdapter.stop();
            console.log("[Bridge] Scheduler adapter stopped");
          } catch (err) {
            console.error("[Bridge] Error stopping scheduler adapter:", err);
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
