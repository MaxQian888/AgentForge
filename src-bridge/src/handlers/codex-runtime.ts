import { calculateCost } from "../cost/calculator.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";

type UnknownRecord = Record<string, unknown>;
type EventSink = Pick<EventStreamer, "send">;

export interface CodexAuthStatus {
  authenticated: boolean;
  message?: string;
}

export type CodexAuthStatusProvider = () => CodexAuthStatus;

export type CodexRuntimeRunner = (params: {
  mode: "start" | "resume";
  command: string;
  req: ExecuteRequest;
  systemPrompt: string;
  prompt: string;
  threadId?: string;
  abortSignal: AbortSignal;
}) => AsyncIterable<UnknownRecord>;

export interface CodexRuntimeDeps {
  command: string;
  codexRuntimeRunner?: CodexRuntimeRunner;
  now?: () => number;
}

export async function streamCodexRuntime(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  systemPrompt: string,
  deps: CodexRuntimeDeps,
): Promise<void> {
  const runner = deps.codexRuntimeRunner ?? spawnCodexRuntime;
  const now = deps.now ?? Date.now;
  const continuity =
    runtime.continuity?.runtime === "codex" && runtime.continuity.resume_ready
      ? runtime.continuity
      : null;
  const mode = continuity?.thread_id ? "resume" : "start";
  const prompt =
    mode === "resume" ? buildCodexResumePrompt(req.prompt) : buildCodexPrompt(req.prompt, systemPrompt);

  for await (const event of runner({
    mode,
    command: deps.command,
    req,
    systemPrompt,
    prompt,
    threadId: continuity?.thread_id,
    abortSignal: runtime.abortController.signal,
  })) {
    runtime.lastActivity = now();
    emitCodexRuntimeEvent(runtime, streamer, event, req, now);

    if (runtime.spentUsd >= req.budget_usd) {
      runtime.abortController.abort("budget_exceeded");
      throw new Error(`budget exceeded for task ${req.task_id}`);
    }
  }
}

export function getDefaultCodexAuthStatus(command = "codex"): CodexAuthStatus {
  const result = Bun.spawnSync({
    cmd: [command, "login", "status"],
    stdout: "pipe",
    stderr: "pipe",
  });
  const output = `${Buffer.from(result.stdout).toString("utf8")}\n${Buffer.from(result.stderr).toString("utf8")}`.trim();

  if (result.exitCode !== 0) {
    return {
      authenticated: false,
      message: output || "Codex CLI authentication is unavailable",
    };
  }

  if (/logged in/i.test(output)) {
    return {
      authenticated: true,
      message: output,
    };
  }

  return {
    authenticated: false,
    message: output || "Codex CLI authentication is unavailable",
  };
}

function emitCodexRuntimeEvent(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  switch (event.type) {
    case "thread.started":
      if (typeof event.thread_id === "string" && event.thread_id.length > 0) {
        runtime.continuity = {
          runtime: "codex",
          resume_ready: true,
          captured_at: now(),
          thread_id: event.thread_id,
        };
      }
      return;
    case "item.started":
      emitCodexItemStarted(runtime, streamer, event, req, now);
      return;
    case "item.completed":
      emitCodexItemCompleted(runtime, streamer, event, req, now);
      return;
    case "turn.completed":
      emitCodexTurnCompleted(runtime, streamer, event, req, now);
      return;
    case "error":
      if (typeof event.message === "string") {
        throw new Error(event.message);
      }
      return;
    default:
      return;
  }
}

function emitCodexItemStarted(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (!isRecord(event.item) || event.item.type !== "command_execution") {
    return;
  }

  runtime.turnNumber += 1;
  runtime.lastTool = "shell";
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "tool_call",
    data: {
      tool_name: "shell",
      tool_input: JSON.stringify({
        command: typeof event.item.command === "string" ? event.item.command : "",
      }),
      call_id: typeof event.item.id === "string" ? event.item.id : "",
    },
  });
}

function emitCodexItemCompleted(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (!isRecord(event.item)) {
    return;
  }

  if (event.item.type === "agent_message" && typeof event.item.text === "string") {
    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: now(),
      type: "output",
      data: {
        content: event.item.text,
        content_type: "text",
        turn_number: runtime.turnNumber,
      },
    });
    return;
  }

  if (event.item.type !== "command_execution") {
    return;
  }

  const exitCode =
    typeof event.item.exit_code === "number" && Number.isFinite(event.item.exit_code)
      ? event.item.exit_code
      : 0;
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "tool_result",
    data: {
      call_id: typeof event.item.id === "string" ? event.item.id : "",
      output:
        typeof event.item.aggregated_output === "string" ? event.item.aggregated_output : "",
      is_error: exitCode !== 0,
    },
  });
}

function emitCodexTurnCompleted(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  const usage = isRecord(event.usage) ? event.usage : {};
  const inputTokens =
    typeof usage.input_tokens === "number" ? Math.max(usage.input_tokens, 0) : 0;
  const outputTokens =
    typeof usage.output_tokens === "number" ? Math.max(usage.output_tokens, 0) : 0;
  const cacheReadTokens =
    typeof usage.cached_input_tokens === "number" ? Math.max(usage.cached_input_tokens, 0) : 0;
  const reportedTotal =
    typeof event.total_cost_usd === "number" && Number.isFinite(event.total_cost_usd)
      ? Math.max(event.total_cost_usd, 0)
      : undefined;

  runtime.spentUsd =
    reportedTotal ??
    runtime.spentUsd +
      calculateCost(
        {
          input_tokens: inputTokens,
          output_tokens: outputTokens,
          cache_read_input_tokens: cacheReadTokens,
        },
        req.model,
      );

  if (runtime.continuity?.runtime === "codex") {
    runtime.continuity = {
      ...runtime.continuity,
      captured_at: now(),
    };
  }

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

async function* spawnCodexRuntime(params: {
  mode: "start" | "resume";
  command: string;
  req: ExecuteRequest;
  systemPrompt: string;
  prompt: string;
  threadId?: string;
  abortSignal: AbortSignal;
}): AsyncGenerator<UnknownRecord, void> {
  const proc = Bun.spawn({
    cmd: buildCodexCommand(params),
    cwd: params.req.worktree_path,
    env: {
      ...process.env,
      AGENTFORGE_RUNTIME: "codex",
      AGENTFORGE_MODEL: params.req.model ?? "",
      AGENTFORGE_PERMISSION_MODE: params.req.permission_mode,
    },
    stdout: "pipe",
    stderr: "pipe",
  });

  const onAbort = () => {
    try {
      proc.kill();
    } catch {
      // Best effort only.
    }
  };
  params.abortSignal.addEventListener("abort", onAbort, { once: true });

  try {
    if (proc.stdout) {
      for await (const line of readLines(proc.stdout)) {
        if (!line.trim()) {
          continue;
        }

        try {
          const parsed = JSON.parse(line);
          if (isRecord(parsed)) {
            yield parsed;
            continue;
          }
        } catch {
          yield {
            type: "item.completed",
            item: {
              id: "agent-output",
              type: "agent_message",
              text: line,
            },
          };
        }
      }
    }

    const exitCode = await proc.exited;
    const stderr = proc.stderr ? await readToString(proc.stderr) : "";
    if (exitCode !== 0 && !params.abortSignal.aborted) {
      throw new Error(stderr.trim() || `codex runtime exited with code ${exitCode}`);
    }
  } finally {
    params.abortSignal.removeEventListener("abort", onAbort);
  }
}

function buildCodexCommand(params: {
  mode: "start" | "resume";
  command: string;
  req: ExecuteRequest;
  prompt: string;
  threadId?: string;
}): string[] {
  const cmd = [params.command, "-C", params.req.worktree_path, "exec"];

  if (params.mode === "resume") {
    cmd.push("resume");
  }

  cmd.push("--json");

  if (params.req.model) {
    cmd.push("--model", params.req.model);
  }

  if (params.req.permission_mode === "bypassPermissions") {
    cmd.push("--dangerously-bypass-approvals-and-sandbox");
  } else {
    cmd.push("--full-auto");
  }

  if (params.mode === "resume" && params.threadId) {
    cmd.push(params.threadId);
  }

  cmd.push(params.prompt);
  return cmd;
}

function buildCodexPrompt(prompt: string, systemPrompt: string): string {
  if (!systemPrompt.trim()) {
    return prompt;
  }

  return `${systemPrompt}\n\nTask:\n${prompt}`;
}

function buildCodexResumePrompt(prompt: string): string {
  return `Continue the existing task from the saved Codex thread state. Preserve prior context and continue the unfinished work.\n\nOriginal task:\n${prompt}`;
}

async function* readLines(
  stream: ReadableStream<Uint8Array>,
): AsyncGenerator<string, void, undefined> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffered = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      buffered += decoder.decode(value, { stream: true });
      const lines = buffered.split(/\r?\n/);
      buffered = lines.pop() ?? "";
      for (const line of lines) {
        yield line;
      }
    }
  } finally {
    reader.releaseLock();
  }

  const tail = buffered + decoder.decode();
  if (tail.length > 0) {
    yield tail;
  }
}

async function readToString(stream: ReadableStream<Uint8Array>): Promise<string> {
  let output = "";
  for await (const line of readLines(stream)) {
    output += line;
  }
  return output;
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === "object" && value !== null;
}
