import type { RuntimePoolManager } from "../runtime/pool-manager.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { EventStreamer } from "../ws/event-stream.js";
import type { ExecuteRequest, AgentEvent } from "../types.js";
import { buildSystemPrompt } from "../role/injector.js";
import { classifyError } from "./errors.js";

function defaultSystemPrompt(taskId: string): string {
  return `You are a coding agent working on task ${taskId}. Follow best practices and write clean, well-tested code.`;
}

export async function handleExecute(
  pool: RuntimePoolManager,
  streamer: EventStreamer,
  req: ExecuteRequest,
): Promise<{ session_id: string }> {
  const runtime = pool.acquire(req.task_id, req.session_id);

  const systemPrompt = buildSystemPrompt(
    req.system_prompt || defaultSystemPrompt(req.task_id),
    req.role_config,
  );

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: Date.now(),
    type: "status_change",
    data: { old_status: "idle", new_status: "starting" },
  });

  // Fire-and-forget: start async execution
  executeAgent(runtime, streamer, req, systemPrompt).catch((err) => {
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: Date.now(),
      type: "error",
      data: { code: "INTERNAL", message: String(err), retryable: false },
    });
    pool.release(req.task_id);
  });

  return { session_id: req.session_id };
}

async function executeAgent(
  runtime: AgentRuntime,
  streamer: EventStreamer,
  req: ExecuteRequest,
  _systemPrompt: string,
): Promise<void> {
  runtime.status = "running";
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: Date.now(),
    type: "status_change",
    data: { old_status: "starting", new_status: "running" },
  });

  try {
    // TODO: Replace with real Claude Agent SDK query() call
    await simulateAgentExecution(runtime, streamer, req);

    runtime.status = "completed";
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: Date.now(),
      type: "status_change",
      data: { old_status: "running", new_status: "completed", reason: "end_turn" },
    });
  } catch (err: unknown) {
    runtime.status = "failed";
    const classified = classifyError(err);
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: Date.now(),
      type: "error",
      data: classified,
    });
  }
}

async function simulateAgentExecution(
  runtime: AgentRuntime,
  streamer: EventStreamer,
  req: ExecuteRequest,
): Promise<void> {
  const steps: Array<{ type: AgentEvent["type"]; data: unknown }> = [
    {
      type: "output",
      data: {
        content: `Analyzing task: ${req.prompt}`,
        content_type: "text",
        turn_number: 0,
      },
    },
    {
      type: "tool_call",
      data: {
        tool_name: "Read",
        tool_input: JSON.stringify({ file_path: "README.md" }),
        call_id: "call_1",
      },
    },
    {
      type: "tool_result",
      data: {
        call_id: "call_1",
        output: "# Project README...",
        is_error: false,
      },
    },
    {
      type: "output",
      data: {
        content: "I've analyzed the codebase. Now implementing the changes...",
        content_type: "text",
        turn_number: 1,
      },
    },
    {
      type: "cost_update",
      data: {
        input_tokens: 5000,
        output_tokens: 1000,
        cache_read_tokens: 0,
        cost_usd: 0.03,
        budget_remaining_usd: req.budget_usd - 0.03,
      },
    },
  ];

  for (const step of steps) {
    if (runtime.abortController.signal.aborted) {
      throw new Error("Cancelled");
    }
    await new Promise((r) => setTimeout(r, 1000));
    runtime.turnNumber++;
    runtime.lastActivity = Date.now();
    if (step.type === "tool_call") {
      const data = step.data as { tool_name: string };
      runtime.lastTool = data.tool_name;
    }
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: Date.now(),
      type: step.type,
      data: step.data,
    });
  }
}
