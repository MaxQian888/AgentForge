import type { RuntimePoolManager } from "../runtime/pool-manager.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import {
  AgentRuntimeRegistry,
  createRuntimeRegistry,
  type AgentRuntimeRegistryOptions,
  type CodexAuthStatusProvider,
  type CodexRuntimeRunner,
} from "../runtime/registry.js";
import type { EventStreamer } from "../ws/event-stream.js";
import type {
  ExecuteRequest,
  RuntimeContinuityState,
  SessionSnapshot,
} from "../types.js";
import { buildSystemPrompt } from "../role/injector.js";
import { classifyError } from "./errors.js";
import type { CommandRuntimeRunner } from "./command-runtime.js";
import type { ToolPluginManager } from "../plugins/tool-plugin-manager.js";
import type { PluginRecord } from "../plugins/types.js";
import type { SessionManager } from "../session/manager.js";
import type { OpenCodeTransport } from "../runtime/opencode-transport.js";
import type { HookCallbackManager } from "../runtime/hook-callback-manager.js";
import {
  serializeCostAccounting,
} from "../cost/accounting.js";

type EventSink = Pick<EventStreamer, "send">;

export interface ExecuteDeps extends AgentRuntimeRegistryOptions {
  awaitCompletion?: boolean;
  commandRuntimeRunner?: CommandRuntimeRunner;
  codexRuntimeRunner?: CodexRuntimeRunner;
  runtimeRegistry?: AgentRuntimeRegistry;
  pluginManager?: ToolPluginManager;
  activePlugins?: PluginRecord[];
  sessionManager?: SessionManager;
  now?: () => number;
  hookCallbackManager?: HookCallbackManager;
  continuity?: RuntimeContinuityState;
  opencodeTransport?: OpenCodeTransport;
  codexAuthStatusProvider?: CodexAuthStatusProvider;
  forkSessionRunner?: AgentRuntimeRegistryOptions["forkSessionRunner"];
  queryRunner?: never; // Removed: legacy claude-runtime queryRunner
  opencodeEventRunner?: never; // Removed: legacy opencode event runner
}

function defaultSystemPrompt(taskId: string): string {
  return `You are a coding agent working on task ${taskId}. Follow best practices and write clean, well-tested code.`;
}

export function buildRuntimeSnapshot(
  runtime: AgentRuntime,
  req: ExecuteRequest,
  now: () => number,
): SessionSnapshot {
  const updatedAt = now();
  return {
    task_id: req.task_id,
    session_id: req.session_id,
    status: runtime.status,
    turn_number: runtime.turnNumber,
    spent_usd: runtime.spentUsd,
    created_at: runtime.createdAt,
    updated_at: updatedAt,
    request: { ...req },
    cost_accounting: serializeCostAccounting(runtime.costAccounting),
    continuity: runtime.continuity
      ? { ...runtime.continuity }
      : req.runtime === "claude_code" || req.provider === "anthropic"
        ? {
            runtime: "claude_code",
            resume_ready: false,
            captured_at: updatedAt,
            blocking_reason: "missing_continuity_state",
          }
        : req.runtime === "codex" ||
            req.provider === "codex" ||
            req.provider === "openai"
          ? {
              runtime: "codex",
              resume_ready: false,
              captured_at: updatedAt,
              blocking_reason: "missing_continuity_state",
            }
          : req.runtime === "opencode" || req.provider === "opencode"
            ? {
                runtime: "opencode",
                resume_ready: false,
                captured_at: updatedAt,
                blocking_reason: "missing_continuity_state",
              }
            : undefined,
  };
}

export function persistRuntimeSnapshot(
  runtime: AgentRuntime,
  req: ExecuteRequest,
  streamer: EventSink,
  sessionManager: SessionManager | undefined,
  now: () => number,
): void {
  const snapshot = buildRuntimeSnapshot(runtime, req, now);

  sessionManager?.save(req.task_id, snapshot);

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: snapshot.updated_at,
    type: "snapshot",
    data: snapshot,
  });
}

export async function handleExecute(
  pool: RuntimePoolManager,
  streamer: EventSink,
  req: ExecuteRequest,
  deps: ExecuteDeps = {},
): Promise<{ session_id: string }> {
  // Resolve active MCP tool plugins for agent execution before adapter construction.
  if (deps.pluginManager) {
    const toolPluginIds = req.role_config?.tools ?? [];
    const allPlugins = deps.pluginManager.list();
    deps.activePlugins = allPlugins.filter(
      (p) =>
        p.lifecycle_state === "active" &&
        (toolPluginIds.length === 0 || toolPluginIds.includes(p.metadata.id)),
    );
  }

  const runtimeRegistry =
    deps.runtimeRegistry ??
    createRuntimeRegistry({
      commandRuntimeRunner: deps.commandRuntimeRunner,
      codexRuntimeRunner: deps.codexRuntimeRunner,
      activePlugins: deps.activePlugins,
      forkSessionRunner: deps.forkSessionRunner,
      executableLookup: deps.executableLookup,
      envLookup: deps.envLookup,
      defaultRuntime: deps.defaultRuntime,
      now: deps.now,
      codexAuthStatusProvider: deps.codexAuthStatusProvider,
      opencodeTransport: deps.opencodeTransport,
    });
  const { adapter, request } = await runtimeRegistry.resolveExecute(req);
  const runtime = pool.acquire(request.task_id, request.session_id, request.runtime ?? "claude_code");
  runtime.bindRequest(request);
  runtime.continuity = deps.continuity ? { ...deps.continuity } : runtime.continuity;

  // Apply role enforcement limits
  if (request.role_config) {
    runtime.applyRoleLimits(request.role_config);
  }

  let baseSystemPrompt = request.system_prompt || defaultSystemPrompt(request.task_id);
  if (request.team_context) {
    baseSystemPrompt = `## Team Context\n\n${request.team_context}\n\n${baseSystemPrompt}`;
  }
  const systemPrompt = buildSystemPrompt(
    baseSystemPrompt,
    request.role_config,
  );

  streamer.send({
    task_id: request.task_id,
    session_id: request.session_id,
    timestamp_ms: Date.now(),
    type: "status_change",
    data: { old_status: "idle", new_status: "starting" },
  });

  const work = executeAgent(runtime, streamer, request, systemPrompt, adapter, deps).finally(() => {
    pool.release(request.task_id);
  });

  if (deps.awaitCompletion) {
    await work;
  } else {
    work.catch((err) => {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: (deps.now ?? Date.now)(),
        type: "error",
        data: { code: "INTERNAL", message: String(err), retryable: false },
      });
    });
  }

  return { session_id: request.session_id };
}

async function executeAgent(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  systemPrompt: string,
  adapter: {
    execute(
      runtime: AgentRuntime,
      streamer: EventSink,
      req: ExecuteRequest,
      systemPrompt: string,
    ): Promise<void>;
  },
  deps: ExecuteDeps,
): Promise<void> {
  const now = deps.now ?? Date.now;
  runtime.status = "running";
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "status_change",
    data: { old_status: "starting", new_status: "running" },
  });

  // Auto session snapshots every 5 minutes
  const autoSnapshotInterval = setInterval(() => {
    if (runtime.status === "running") {
      persistRuntimeSnapshot(runtime, req, streamer, deps.sessionManager, now);
    }
  }, 300_000);

  try {
    await adapter.execute(runtime, streamer, req, systemPrompt);

    runtime.status = "completed";
    persistRuntimeSnapshot(runtime, req, streamer, deps.sessionManager, now);
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "status_change",
      data: {
        old_status: "running",
        new_status: "completed",
        reason: "end_turn",
        structured_output: runtime.structuredOutput ?? undefined,
      },
    });
  } catch (err: unknown) {
    runtime.status = classifyTerminalStatus(runtime, err);
    const classified = classifyError(err);
    if (runtime.status !== "paused") {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "error",
        data: classified,
      });
    }
    persistRuntimeSnapshot(runtime, req, streamer, deps.sessionManager, now);
    const reason =
      runtime.status === "paused"
        ? "user_requested_pause"
        : runtime.status === "budget_exceeded"
          ? "budget_exceeded"
          : runtime.status === "cancelled"
            ? "cancelled_by_user"
            : "runtime_error";
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "status_change",
      data: { old_status: "running", new_status: runtime.status, reason },
    });
  } finally {
    clearInterval(autoSnapshotInterval);
  }
}

function classifyTerminalStatus(
  runtime: AgentRuntime,
  err: unknown,
): AgentRuntime["status"] {
  if (runtime.status === "paused") {
    return "paused";
  }

  const reason = runtime.abortController.signal.reason;
  const message = err instanceof Error ? err.message : String(err);

  if (reason === "budget_exceeded" || message.includes("budget exceeded")) {
    return "budget_exceeded";
  }
  if (reason === "turn_limit_exceeded" || message.includes("turn limit exceeded")) {
    return "failed";
  }
  if (reason === "cancelled_by_user") {
    return "cancelled";
  }
  return "failed";
}
