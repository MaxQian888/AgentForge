import { query, type Options } from "@anthropic-ai/claude-agent-sdk";
import { calculateCost, type UsageInfo } from "../cost/calculator.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { SessionManager } from "../session/manager.js";
import type { ExecuteRequest } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";

type UnknownRecord = Record<string, unknown>;
type EventSink = Pick<EventStreamer, "send">;

export type QueryRunner = (params: {
  prompt: string;
  options?: Record<string, unknown>;
}) => AsyncIterable<UnknownRecord>;

export interface ClaudeRuntimeDeps {
  queryRunner?: QueryRunner;
  sessionManager?: SessionManager;
  now?: () => number;
}

export function buildClaudeQueryOptions(
  req: ExecuteRequest,
  systemPrompt: string,
  runtime: AgentRuntime,
): Options & Record<string, unknown> {
  const options: Options & Record<string, unknown> = {
    abortController: runtime.abortController,
    allowedTools: req.allowed_tools.length > 0 ? req.allowed_tools : undefined,
    cwd: req.worktree_path,
    maxTurns: req.max_turns,
    permissionMode: req.permission_mode as Options["permissionMode"],
    systemPrompt,
  };

  if (req.permission_mode === "bypassPermissions") {
    options.allowDangerouslySkipPermissions = true;
  }

  return options;
}

export async function streamClaudeRuntime(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  systemPrompt: string,
  deps: ClaudeRuntimeDeps = {},
): Promise<void> {
  const queryRunner = deps.queryRunner ?? ((query as unknown) as QueryRunner);
  const now = deps.now ?? Date.now;
  const options = buildClaudeQueryOptions(req, systemPrompt, runtime);

  for await (const message of queryRunner({
    prompt: req.prompt,
    options,
  })) {
    runtime.lastActivity = now();

    emitAssistantBlocks(runtime, streamer, message, req, now);
    emitToolResult(streamer, message, req, now);
    emitUsage(runtime, streamer, message, req, now);

    if (runtime.spentUsd >= req.budget_usd) {
      runtime.abortController.abort("budget_exceeded");
      throw new Error(`budget exceeded for task ${req.task_id}`);
    }
  }
}

export function persistRuntimeSnapshot(
  runtime: AgentRuntime,
  req: ExecuteRequest,
  streamer: EventSink,
  sessionManager: SessionManager | undefined,
  now: () => number,
): void {
  const snapshot = {
    task_id: req.task_id,
    session_id: req.session_id,
    status: runtime.status,
    turn_number: runtime.turnNumber,
    spent_usd: runtime.spentUsd,
    created_at: runtime.createdAt,
    updated_at: now(),
  };

  sessionManager?.save(req.task_id, snapshot);

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: snapshot.updated_at,
    type: "snapshot",
    data: snapshot,
  });
}

function emitAssistantBlocks(
  runtime: AgentRuntime,
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (message.type !== "assistant") return;

  const contentBlocks = getContentBlocks(message);
  if (!contentBlocks) return;

  for (const block of contentBlocks) {
    if (block.type === "text" && typeof block.text === "string") {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "output",
        data: {
          content: block.text,
          content_type: "text",
          turn_number: runtime.turnNumber,
        },
      });
      continue;
    }

    if (block.type === "tool_use" && typeof block.name === "string") {
      runtime.turnNumber += 1;
      runtime.lastTool = block.name;
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "tool_call",
        data: {
          tool_name: block.name,
          tool_input: JSON.stringify(block.input ?? {}),
          call_id: typeof block.id === "string" ? block.id : "",
        },
      });
    }
  }
}

function emitToolResult(
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (message.type !== "user" || !isRecord(message.tool_use_result)) return;

  const result = message.tool_use_result;
  const callId =
    typeof result.tool_use_id === "string"
      ? result.tool_use_id
      : typeof message.parent_tool_use_id === "string"
        ? message.parent_tool_use_id
        : "";

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "tool_result",
    data: {
      call_id: callId,
      output: formatToolResultOutput(result),
      is_error: Boolean(result.is_error),
    },
  });
}

function emitUsage(
  runtime: AgentRuntime,
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  const usage = extractUsage(message);
  if (!usage) return;

  const reportedTotal =
    typeof message.total_cost_usd === "number" && Number.isFinite(message.total_cost_usd)
      ? message.total_cost_usd
      : undefined;
  const nextSpent =
    reportedTotal !== undefined ? reportedTotal : runtime.spentUsd + calculateCost(usage);
  const incrementalCost = Math.max(nextSpent - runtime.spentUsd, 0);
  runtime.spentUsd = nextSpent;

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "cost_update",
    data: {
      input_tokens: usage.input_tokens ?? 0,
      output_tokens: usage.output_tokens ?? 0,
      cache_read_tokens: usage.cache_read_input_tokens ?? 0,
      cost_usd: incrementalCost,
      budget_remaining_usd: Math.max(req.budget_usd - runtime.spentUsd, 0),
    },
  });
}

function extractUsage(message: UnknownRecord): UsageInfo | null {
  if (!isRecord(message.usage)) return null;

  return {
    input_tokens:
      typeof message.usage.input_tokens === "number" ? message.usage.input_tokens : undefined,
    output_tokens:
      typeof message.usage.output_tokens === "number" ? message.usage.output_tokens : undefined,
    cache_read_input_tokens:
      typeof message.usage.cache_read_input_tokens === "number"
        ? message.usage.cache_read_input_tokens
        : undefined,
  };
}

function getContentBlocks(message: UnknownRecord): Array<UnknownRecord> | null {
  if (!isRecord(message.message) || !Array.isArray(message.message.content)) return null;

  return message.message.content.filter(isRecord);
}

function formatToolResultOutput(result: UnknownRecord): string {
  if (typeof result.output === "string") {
    return result.output;
  }

  if (typeof result.content === "string") {
    return result.content;
  }

  return JSON.stringify(result);
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === "object" && value !== null;
}
