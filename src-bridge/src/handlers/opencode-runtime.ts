import { calculateCost } from "../cost/calculator.js";
import type { OpenCodeTransport } from "../opencode/transport.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";

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
    runtime.continuity?.runtime === "opencode" && runtime.continuity.resume_ready
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
    throw new Error(String(runtime.abortController.signal.reason ?? "opencode run aborted"));
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
  if (eventName === "session.error") {
    throw new Error(getErrorMessage(data) || `OpenCode session ${sessionId} failed`);
  }

  if (eventName === "message.part.delta" || eventName === "message.part.updated") {
    const part = isRecord(data.part) ? data.part : null;
    if (!part) {
      return false;
    }
    updateOpenCodeContinuity(runtime, sessionId, now, getLatestMessageID(data));

    if (part.type === "text" && typeof part.text === "string" && part.text.length > 0) {
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

    if (part.type === "tool") {
      const toolId = typeof part.id === "string" ? part.id : "";
      const toolName = typeof part.toolName === "string" ? part.toolName : "tool";
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
              typeof part.output === "string" ? part.output : JSON.stringify(part.output ?? {}),
            is_error: toolState === "error" || Boolean(part.isError),
          },
        });
      }
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
    typeof usage.input_tokens === "number" ? Math.max(usage.input_tokens, 0) : 0;
  const outputTokens =
    typeof usage.output_tokens === "number" ? Math.max(usage.output_tokens, 0) : 0;
  const cacheReadTokens =
    typeof usage.cached_input_tokens === "number" ? Math.max(usage.cached_input_tokens, 0) : 0;
  const nextSpent =
    typeof data.total_cost_usd === "number" && Number.isFinite(data.total_cost_usd)
      ? Math.max(data.total_cost_usd, 0)
      : runtime.spentUsd +
        calculateCost(
          {
            input_tokens: inputTokens,
            output_tokens: outputTokens,
            cache_read_input_tokens: cacheReadTokens,
          },
          req.model,
        );

  runtime.spentUsd = nextSpent;
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
    },
  });
}

function updateOpenCodeContinuity(
  runtime: AgentRuntime,
  sessionId: string,
  now: () => number,
  latestMessageId?: string,
): void {
  const previous =
    runtime.continuity?.runtime === "opencode" ? runtime.continuity : undefined;
  runtime.continuity = {
    runtime: "opencode",
    resume_ready: true,
    captured_at: now(),
    upstream_session_id: sessionId,
    latest_message_id: latestMessageId ?? previous?.latest_message_id,
    server_url: previous?.server_url,
  };
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
  if (isRecord(data.message) && typeof data.message.id === "string") return data.message.id;
  return undefined;
}

function getErrorMessage(data: UnknownRecord): string | undefined {
  if (isRecord(data.error)) {
    if (typeof data.error.message === "string") return data.error.message;
    if (isRecord(data.error.data) && typeof data.error.data.message === "string") {
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
