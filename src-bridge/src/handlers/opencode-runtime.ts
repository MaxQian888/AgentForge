import {
  accumulateCostAccounting,
  serializeCostAccounting,
} from "../cost/accounting.js";
import type { OpenCodeTransport } from "../opencode/transport.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";
import { emitBudgetAlertIfNeeded } from "./budget-events.js";

type UnknownRecord = Record<string, unknown>;
type EventSink = Pick<EventStreamer, "send">;

export type OpenCodeEventRunner = (params: {
  mode: "start" | "resume";
  transport: OpenCodeTransport;
  sessionId: string;
  req: ExecuteRequest;
  systemPrompt: string;
  prompt: string;
  abortSignal: AbortSignal;
}) => AsyncIterable<UnknownRecord>;

export interface OpenCodeRuntimeDeps {
  transport: OpenCodeTransport;
  eventRunner?: OpenCodeEventRunner;
  now?: () => number;
}

export async function streamOpenCodeRuntime(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  systemPrompt: string,
  deps: OpenCodeRuntimeDeps,
): Promise<void> {
  const now = deps.now ?? Date.now;
  const continuity =
    runtime.continuity?.runtime === "opencode" &&
    runtime.continuity.resume_ready
      ? runtime.continuity
      : null;
  const mode = continuity?.upstream_session_id ? "resume" : "start";
  const sessionId =
    continuity?.upstream_session_id ??
    (await deps.transport.createSession({ title: req.task_id })).id;

  runtime.continuity = {
    runtime: "opencode",
    resume_ready: true,
    captured_at: now(),
    upstream_session_id: sessionId,
    latest_message_id: continuity?.latest_message_id,
    server_url: deps.transport.serverUrl,
    fork_available: true,
    revert_message_ids: continuity?.revert_message_ids ?? [],
  };

  const prompt = mode === "resume" ? buildResumePrompt(req.prompt) : req.prompt;
  await deps.transport.sendPromptAsync({
    sessionId,
    provider: req.provider ?? "opencode",
    model: req.model,
    prompt,
  });

  const runner = deps.eventRunner ?? defaultOpenCodeEventRunner;
  let sawTerminal = false;
  for await (const event of runner({
    mode,
    transport: deps.transport,
    sessionId,
    req,
    systemPrompt,
    prompt,
    abortSignal: runtime.abortController.signal,
  })) {
    runtime.lastActivity = now();
    if (emitOpenCodeEvent(runtime, streamer, event, req, now, sessionId)) {
      sawTerminal = true;
      return;
    }

    if (runtime.spentUsd >= req.budget_usd) {
      runtime.abortController.abort("budget_exceeded");
      throw new Error(`budget exceeded for task ${req.task_id}`);
    }
  }

  if (runtime.abortController.signal.aborted) {
    throw new Error(
      String(runtime.abortController.signal.reason ?? "opencode run aborted"),
    );
  }
  if (!sawTerminal) {
    throw new Error("OpenCode event stream ended before session became idle");
  }
}

async function* defaultOpenCodeEventRunner(params: {
  transport: OpenCodeTransport;
  abortSignal: AbortSignal;
}): AsyncGenerator<UnknownRecord, void> {
  for await (const event of params.transport.streamEvents(params.abortSignal)) {
    yield event;
  }
}

function emitOpenCodeEvent(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
  sessionId: string,
): boolean {
  const eventName = typeof event.event === "string" ? event.event : "";
  const data = isRecord(event.data) ? event.data : event;
  if (!matchesSession(data, sessionId)) {
    return false;
  }

  if (eventName === "session.idle") {
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));
    return true;
  }
  if (eventName === "session.status") {
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "status_change",
      data: {
        state: mapOpenCodeSessionStatus(
          typeof data.status === "string" ? data.status : undefined,
        ),
      },
    });
    return false;
  }
  if (eventName === "session.error") {
    throw new Error(
      getErrorMessage(data) || `OpenCode session ${sessionId} failed`,
    );
  }
  if (eventName === "todo.updated") {
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "todo_update",
      data: {
        todos: Array.isArray(data.todos) ? data.todos : [],
      },
    });
    return false;
  }
  if (eventName === "message.updated") {
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));
    emitOpenCodeMessageUpdate(streamer, req, now, data);
    return false;
  }
  if (eventName === "command.executed") {
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "output",
      data: {
        content: `Command /${typeof data.name === "string" ? data.name : "unknown"} executed`,
        content_type: "text",
        turn_number: runtime.turnNumber,
      },
    });
    return false;
  }
  if (eventName === "vcs.branch.updated") {
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "status_change",
      data: {
        reason: "branch_updated",
        branch: typeof data.branch === "string" ? data.branch : undefined,
      },
    });
    return false;
  }

  if (
    eventName === "message.part.delta" ||
    eventName === "message.part.updated"
  ) {
    const part = isRecord(data.part) ? data.part : null;
    if (!part) {
      return false;
    }
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));

    const partType = normalizeOpenCodePartType(part);

    if (
      partType === "text" &&
      typeof part.text === "string" &&
      part.text.length > 0
    ) {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "output",
        data: {
          content: part.text,
          content_type: "text",
          turn_number: runtime.turnNumber,
        },
      });
      return false;
    }

    if (partType === "tool") {
      const toolId = typeof part.id === "string" ? part.id : "";
      const toolName =
        typeof part.toolName === "string" ? part.toolName : "tool";
      const toolState = typeof part.state === "string" ? part.state : "";
      if (toolState === "running" || toolState === "pending") {
        runtime.turnNumber += 1;
        runtime.lastTool = toolName;
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "tool_call",
          data: {
            tool_name: toolName,
            tool_input: JSON.stringify(part.input ?? {}),
            call_id: toolId,
          },
        });
      } else if (toolState === "completed" || toolState === "error") {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "tool_result",
          data: {
            call_id: toolId,
            output:
              typeof part.output === "string"
                ? part.output
                : JSON.stringify(part.output ?? {}),
            is_error: toolState === "error" || Boolean(part.isError),
          },
        });
      }
      return false;
    }

    if (partType === "reasoning" && typeof part.reasoning === "string") {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "reasoning",
        data: {
          content: part.reasoning,
        },
      });
      return false;
    }

    if (partType === "file") {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "file_change",
        data: {
          files: Array.isArray(part.files) ? part.files : [],
        },
      });
      return false;
    }

    if (partType === "agent") {
      const content =
        typeof part.output === "string"
          ? part.output
          : typeof part.content === "string"
            ? part.content
            : undefined;
      if (content) {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "output",
          data: {
            content,
            content_type: "text",
            turn_number: runtime.turnNumber,
            agent_name:
              typeof part.agentName === "string" ? part.agentName : undefined,
          },
        });
      }
      return false;
    }

    if (partType === "compaction") {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "status_change",
        data: {
          reason: "compaction",
          summary: typeof part.summary === "string" ? part.summary : undefined,
        },
      });
      return false;
    }

    if (partType === "subtask") {
      const title = typeof part.title === "string" ? part.title : "subtask";
      const content = typeof part.content === "string" ? part.content : "";
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "output",
        data: {
          content: `Subtask ${title}: ${content}`.trim(),
          content_type: "text",
          turn_number: runtime.turnNumber,
        },
      });
      return false;
    }
  }

  if (eventName === "session.updated") {
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));
    emitUsage(runtime, streamer, data, req, now);
  }

  return false;
}

function emitUsage(
  runtime: AgentRuntime,
  streamer: EventSink,
  data: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  const usage = isRecord(data.usage) ? data.usage : {};
  const inputTokens =
    typeof usage.input_tokens === "number"
      ? Math.max(usage.input_tokens, 0)
      : 0;
  const outputTokens =
    typeof usage.output_tokens === "number"
      ? Math.max(usage.output_tokens, 0)
      : 0;
  const cacheReadTokens =
    typeof usage.cached_input_tokens === "number"
      ? Math.max(usage.cached_input_tokens, 0)
      : 0;
  const nextSpent =
    typeof data.total_cost_usd === "number" &&
    Number.isFinite(data.total_cost_usd)
      ? Math.max(data.total_cost_usd, 0)
      : undefined;
  const snapshot = accumulateCostAccounting({
    previous: runtime.costAccounting,
    runtime: req.runtime ?? "opencode",
    provider: req.provider ?? "opencode",
    requestedModel: req.model,
    usageDelta: {
      inputTokens,
      outputTokens,
      cacheReadTokens,
    },
    authoritativeTotalCostUsd: nextSpent,
    source:
      nextSpent !== undefined ? "opencode_native_total" : "opencode_usage",
  });
  runtime.applyCostAccounting(snapshot);
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "cost_update",
    data: {
      input_tokens: inputTokens,
      output_tokens: outputTokens,
      cache_read_tokens: cacheReadTokens,
      cost_usd: runtime.spentUsd,
      budget_remaining_usd: Math.max(req.budget_usd - runtime.spentUsd, 0),
      turn_number: runtime.turnNumber,
      cost_accounting: serializeCostAccounting(snapshot),
    },
  });
  emitBudgetAlertIfNeeded(runtime, streamer, req, now);
}

function updateOpenCodeContinuity(
  runtime: AgentRuntime,
  sessionId: string,
  now: () => number,
  latestMessageId?: string,
): void {
  const previous =
    runtime.continuity?.runtime === "opencode" ? runtime.continuity : undefined;
  const revertIds = new Set(previous?.revert_message_ids ?? []);
  if (latestMessageId) {
    revertIds.add(latestMessageId);
  }
  runtime.continuity = {
    runtime: "opencode",
    resume_ready: true,
    captured_at: now(),
    upstream_session_id: sessionId,
    latest_message_id: latestMessageId ?? previous?.latest_message_id,
    server_url: previous?.server_url,
    fork_available: true,
    revert_message_ids: Array.from(revertIds),
  };
}

function emitOpenCodeMessageUpdate(
  streamer: EventSink,
  req: ExecuteRequest,
  now: () => number,
  data: UnknownRecord,
): void {
  const message = isRecord(data.message) ? data.message : null;
  const content =
    typeof message?.content === "string"
      ? message.content
      : Array.isArray(message?.content)
        ? message.content
            .map((part) =>
              isRecord(part) && typeof part.text === "string" ? part.text : "",
            )
            .filter(Boolean)
            .join("\n")
        : null;
  if (!content) {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "output",
    data: {
      content,
      content_type: "text",
    },
  });
}

function normalizeOpenCodePartType(part: UnknownRecord): string {
  const rawType =
    typeof part.type === "string"
      ? part.type
      : typeof part.kind === "string"
        ? part.kind
        : "";
  return rawType.replace(/Part$/, "").toLowerCase();
}

function mapOpenCodeSessionStatus(status: string | undefined): string {
  switch (status) {
    case "busy":
    case "running":
      return "running";
    case "idle":
      return "completed";
    default:
      return status ?? "unknown";
  }
}

function matchesSession(data: UnknownRecord, sessionId: string): boolean {
  return (
    data.sessionID === sessionId ||
    data.sessionId === sessionId ||
    data.session_id === sessionId
  );
}

function getLatestMessageID(data: UnknownRecord): string | undefined {
  if (typeof data.messageID === "string") return data.messageID;
  if (typeof data.messageId === "string") return data.messageId;
  if (isRecord(data.message) && typeof data.message.id === "string")
    return data.message.id;
  return undefined;
}

function getErrorMessage(data: UnknownRecord): string | undefined {
  if (isRecord(data.error)) {
    if (typeof data.error.message === "string") return data.error.message;
    if (
      isRecord(data.error.data) &&
      typeof data.error.data.message === "string"
    ) {
      return data.error.data.message;
    }
  }
  return undefined;
}

function buildResumePrompt(prompt: string): string {
  return `Continue the existing OpenCode session from the saved state. Preserve prior context and continue the unfinished work.\n\nOriginal task:\n${prompt}`;
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === "object" && value !== null;
}
