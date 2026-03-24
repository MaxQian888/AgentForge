import type { RuntimePoolManager } from "../runtime/pool-manager.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import {
  AgentRuntimeRegistry,
  createRuntimeRegistry,
  type AgentRuntimeRegistryOptions,
} from "../runtime/registry.js";
import type { EventStreamer } from "../ws/event-stream.js";
import type { ExecuteRequest } from "../types.js";
import { buildSystemPrompt } from "../role/injector.js";
import { classifyError } from "./errors.js";
import {
  persistRuntimeSnapshot,
  type ClaudeRuntimeDeps,
} from "./claude-runtime.js";
import type { CommandRuntimeRunner } from "./command-runtime.js";

function defaultSystemPrompt(taskId: string): string {
  return `You are a coding agent working on task ${taskId}. Follow best practices and write clean, well-tested code.`;
}

interface ExecuteDeps extends ClaudeRuntimeDeps, AgentRuntimeRegistryOptions {
  awaitCompletion?: boolean;
  commandRuntimeRunner?: CommandRuntimeRunner;
  runtimeRegistry?: AgentRuntimeRegistry;
}

type EventSink = Pick<EventStreamer, "send">;

export async function handleExecute(
  pool: RuntimePoolManager,
  streamer: EventSink,
  req: ExecuteRequest,
  deps: ExecuteDeps = {},
): Promise<{ session_id: string }> {
  const runtimeRegistry =
    deps.runtimeRegistry ??
    createRuntimeRegistry({
      queryRunner: deps.queryRunner,
      commandRuntimeRunner: deps.commandRuntimeRunner,
      executableLookup: deps.executableLookup,
      envLookup: deps.envLookup,
      defaultRuntime: deps.defaultRuntime,
      now: deps.now,
    });
  const { adapter, request } = runtimeRegistry.resolveExecute(req);
  const runtime = pool.acquire(request.task_id, request.session_id);

  const systemPrompt = buildSystemPrompt(
    request.system_prompt || defaultSystemPrompt(request.task_id),
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

  try {
    await adapter.execute(runtime, streamer, req, systemPrompt);

    runtime.status = "completed";
    persistRuntimeSnapshot(runtime, req, streamer, deps.sessionManager, now);
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "status_change",
      data: { old_status: "running", new_status: "completed", reason: "end_turn" },
    });
  } catch (err: unknown) {
    runtime.status = "failed";
    const classified = classifyError(err);
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "error",
      data: classified,
    });
    persistRuntimeSnapshot(runtime, req, streamer, deps.sessionManager, now);
  }
}
