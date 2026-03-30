import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { calculateCost } from "../cost/calculator.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { PluginRecord } from "../plugins/types.js";
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
  activePlugins?: PluginRecord[];
}) => AsyncIterable<UnknownRecord>;

export interface CodexRuntimeDeps {
  command: string;
  codexRuntimeRunner?: CodexRuntimeRunner;
  now?: () => number;
  activePlugins?: PluginRecord[];
}

export interface CodexLaunch {
  cmd: string[];
  env: Record<string, string>;
  cleanup: () => void;
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
    activePlugins: deps.activePlugins,
  })) {
    runtime.lastActivity = now();
    emitCodexRuntimeEvent(runtime, streamer, event, req, now);

    if (runtime.spentUsd >= req.budget_usd) {
      runtime.abortController.abort("budget_exceeded");
      throw new Error(`budget exceeded for task ${req.task_id}`);
    }
  }
}

export function prepareCodexLaunch(params: {
  mode: "start" | "resume";
  command: string;
  req: ExecuteRequest;
  prompt: string;
  threadId?: string;
  activePlugins?: PluginRecord[];
}): CodexLaunch {
  const tempRoot = mkdtempSync(join(tmpdir(), "agentforge-codex-"));
  const cleanup = () => {
    rmSync(tempRoot, { force: true, recursive: true });
  };

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

  if (params.req.output_schema?.schema) {
    const schemaPath = join(tempRoot, "output-schema.json");
    writeFileSync(schemaPath, JSON.stringify(params.req.output_schema.schema, null, 2), "utf8");
    cmd.push("--output-schema", schemaPath);
  }

  for (const attachment of params.req.attachments ?? []) {
    if (attachment.type === "image") {
      cmd.push("--image", attachment.path);
    }
  }

  for (const dir of params.req.additional_directories ?? []) {
    cmd.push("--add-dir", dir);
  }

  if (params.req.web_search) {
    cmd.push("--search");
  }

  for (const configValue of buildCodexConfigOverrides(params.activePlugins ?? [])) {
    cmd.push("-c", configValue);
  }

  if (params.mode === "resume" && params.threadId) {
    cmd.push(params.threadId);
  }

  cmd.push(params.prompt);

  return {
    cmd,
    env: {
      ...Object.fromEntries(
        Object.entries(process.env).map(([key, value]) => [key, value ?? ""]),
      ),
      ...Object.fromEntries(
        Object.entries(params.req.env ?? {}).map(([key, value]) => [key, value ?? ""]),
      ),
      AGENTFORGE_RUNTIME: "codex",
      AGENTFORGE_MODEL: params.req.model ?? "",
      AGENTFORGE_PERMISSION_MODE: params.req.permission_mode,
    },
    cleanup,
  };
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
          fork_available: true,
          rollback_turns: runtime.continuity?.runtime === "codex"
            ? runtime.continuity.rollback_turns ?? 0
            : 0,
        };
      }
      return;
    case "turn.started":
      emitCodexTurnStarted(runtime, streamer, req, now);
      return;
    case "item.started":
      emitCodexItemStarted(runtime, streamer, event, req, now);
      return;
    case "item.updated":
      emitCodexItemUpdated(streamer, event, req, now);
      return;
    case "item.completed":
      emitCodexItemCompleted(runtime, streamer, event, req, now);
      return;
    case "turn.completed":
      emitCodexTurnCompleted(runtime, streamer, event, req, now);
      return;
    case "turn.failed":
      emitCodexTurnFailed(runtime, streamer, event, req, now);
      return;
    case "error":
      emitCodexTopLevelError(runtime, streamer, event, req, now);
      return;
    default:
      return;
  }
}

function emitCodexTurnStarted(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (runtime.continuity?.runtime === "codex") {
    runtime.continuity = {
      ...runtime.continuity,
      captured_at: now(),
      rollback_turns: (runtime.continuity.rollback_turns ?? 0) + 1,
      fork_available: true,
    };
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "status_change",
    data: {
      old_status: "running",
      new_status: "running",
      reason: "turn_started",
    },
  });
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

function emitCodexItemUpdated(
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (!isRecord(event.item)) {
    return;
  }

  const itemType = normalizeCodexItemType(event.item);
  if (itemType !== "command_execution") {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "progress",
    data: {
      item_type: itemType,
      partial_output:
        typeof event.item.aggregated_output === "string"
          ? event.item.aggregated_output
          : "",
      status:
        typeof event.item.status === "string"
          ? event.item.status
          : undefined,
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

  const itemType = normalizeCodexItemType(event.item);

  if (itemType === "agent_message" && typeof event.item.text === "string") {
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

  if (itemType === "command_execution") {
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
    return;
  }

  emitCodexItemDetail(runtime, streamer, event.item, req, now);
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
      fork_available: true,
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

function emitCodexTurnFailed(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): never {
  runtime.status = "failed";
  const message = extractCodexErrorMessage(event) ?? "codex turn failed";
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "error",
    data: {
      message,
      source: "codex",
    },
  });
  throw new Error(message);
}

function emitCodexTopLevelError(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): never {
  runtime.status = "failed";
  const message = extractCodexErrorMessage(event) ?? "codex runtime failed";
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "error",
    data: {
      message,
      source: "codex",
    },
  });
  throw new Error(message);
}

async function* spawnCodexRuntime(params: {
  mode: "start" | "resume";
  command: string;
  req: ExecuteRequest;
  systemPrompt: string;
  prompt: string;
  threadId?: string;
  abortSignal: AbortSignal;
  activePlugins?: PluginRecord[];
}): AsyncGenerator<UnknownRecord, void> {
  const launch = prepareCodexLaunch({
    mode: params.mode,
    command: params.command,
    req: params.req,
    prompt: params.prompt,
    threadId: params.threadId,
    activePlugins: params.activePlugins,
  });
  const proc = Bun.spawn({
    cmd: launch.cmd,
    cwd: params.req.worktree_path,
    env: launch.env,
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
    launch.cleanup();
  }
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

function buildCodexConfigOverrides(activePlugins: PluginRecord[]): string[] {
  const overrides: string[] = [];

  for (const plugin of activePlugins) {
    const id = plugin.metadata.id;
    if (plugin.spec.transport === "http" && plugin.spec.url) {
      overrides.push(`mcp_servers.${id}.url=${tomlString(plugin.spec.url)}`);
      continue;
    }

    if (plugin.spec.command) {
      overrides.push(`mcp_servers.${id}.command=${tomlString(plugin.spec.command)}`);
      if (plugin.spec.args?.length) {
        overrides.push(`mcp_servers.${id}.args=${tomlArray(plugin.spec.args)}`);
      }
      if (plugin.spec.env && Object.keys(plugin.spec.env).length > 0) {
        overrides.push(`mcp_servers.${id}.env=${tomlInlineTable(plugin.spec.env)}`);
      }
    }
  }

  return overrides;
}

function emitCodexItemDetail(
  runtime: AgentRuntime,
  streamer: EventSink,
  item: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  const detail = isRecord(item.details) ? item.details : null;
  if (!detail || typeof detail.type !== "string") {
    return;
  }

  switch (detail.type) {
    case "Reasoning":
      if (typeof detail.summary === "string") {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "reasoning",
          data: {
            content: detail.summary,
          },
        });
      }
      return;
    case "FileChange":
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "file_change",
        data: {
          files: Array.isArray(detail.files) ? detail.files : [],
        },
      });
      return;
    case "McpToolCall":
      emitCodexToolCallResult(
        streamer,
        req,
        now,
        typeof detail.toolName === "string" ? detail.toolName : "mcp_tool",
        detail.input,
        detail.output,
        typeof item.id === "string" ? item.id : "",
      );
      return;
    case "WebSearch":
      emitCodexToolCallResult(
        streamer,
        req,
        now,
        "web_search",
        detail.query,
        detail.results,
        typeof item.id === "string" ? item.id : "",
      );
      return;
    case "TodoList":
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "todo_update",
        data: {
          todos: Array.isArray(detail.todos) ? detail.todos : [],
        },
      });
      return;
    case "CollabToolCall":
      if (typeof detail.output === "string") {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "output",
          data: {
            content: detail.output,
            content_type: "text",
            turn_number: runtime.turnNumber,
            agent_name: detail.agentName,
          },
        });
      }
      return;
    case "Error":
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "error",
        data: {
          message: typeof detail.message === "string" ? detail.message : "codex item error",
          source: "codex",
        },
      });
      return;
    default:
      return;
  }
}

function emitCodexToolCallResult(
  streamer: EventSink,
  req: ExecuteRequest,
  now: () => number,
  toolName: string,
  input: unknown,
  output: unknown,
  callId: string,
): void {
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "tool_call",
    data: {
      tool_name: toolName,
      tool_input: input,
      call_id: callId,
    },
  });
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "tool_result",
    data: {
      call_id: callId,
      output,
      is_error: false,
    },
  });
}

function normalizeCodexItemType(item: UnknownRecord): string {
  if (typeof item.type === "string") {
    return item.type;
  }

  if (isRecord(item.details) && typeof item.details.type === "string") {
    switch (item.details.type) {
      case "AgentMessage":
        return "agent_message";
      case "CommandExecution":
        return "command_execution";
      default:
        return item.details.type;
    }
  }

  return "";
}

function extractCodexErrorMessage(event: UnknownRecord): string | null {
  if (typeof event.message === "string") {
    return event.message;
  }
  if (isRecord(event.error) && typeof event.error.message === "string") {
    return event.error.message;
  }
  return null;
}

function tomlString(value: string): string {
  return JSON.stringify(value);
}

function tomlArray(values: string[]): string {
  return `[${values.map(tomlString).join(",")}]`;
}

function tomlInlineTable(values: Record<string, string>): string {
  return `{${Object.entries(values)
    .map(([key, value]) => `${key}=${tomlString(value)}`)
    .join(",")}}`;
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
