import { calculateCost } from "../cost/calculator.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";

type UnknownRecord = Record<string, unknown>;
type EventSink = Pick<EventStreamer, "send">;
type CommandRuntimeKey = "codex" | "opencode";

export type CommandRuntimeRunner = (params: {
  runtime: CommandRuntimeKey;
  command: string;
  req: ExecuteRequest;
  systemPrompt: string;
  abortSignal: AbortSignal;
}) => AsyncIterable<UnknownRecord>;

export interface CommandRuntimeDeps {
  command: string;
  commandRuntimeRunner?: CommandRuntimeRunner;
  now?: () => number;
}

export async function streamCommandRuntime(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: ExecuteRequest,
  systemPrompt: string,
  deps: CommandRuntimeDeps,
): Promise<void> {
  const runner = deps.commandRuntimeRunner ?? spawnCommandRuntime;
  const now = deps.now ?? Date.now;

  for await (const event of runner({
    runtime: req.runtime as CommandRuntimeKey,
    command: deps.command,
    req,
    systemPrompt,
    abortSignal: runtime.abortController.signal,
  })) {
    runtime.lastActivity = now();

    emitCommandRuntimeEvent(runtime, streamer, event, req, now);

    if (runtime.spentUsd >= req.budget_usd) {
      runtime.abortController.abort("budget_exceeded");
      throw new Error(`budget exceeded for task ${req.task_id}`);
    }
  }
}

function emitCommandRuntimeEvent(
  runtime: AgentRuntime,
  streamer: EventSink,
  event: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  switch (event.type) {
    case "assistant_text":
      if (typeof event.content === "string" && event.content.length > 0) {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "output",
          data: {
            content: event.content,
            content_type: "text",
            turn_number: runtime.turnNumber,
          },
        });
      }
      return;
    case "tool_call":
      if (typeof event.tool_name === "string") {
        runtime.turnNumber += 1;
        runtime.lastTool = event.tool_name;
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "tool_call",
          data: {
            tool_name: event.tool_name,
            tool_input: JSON.stringify(event.tool_input ?? {}),
            call_id: typeof event.call_id === "string" ? event.call_id : "",
          },
        });
      }
      return;
    case "tool_result":
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "tool_result",
        data: {
          call_id: typeof event.call_id === "string" ? event.call_id : "",
          output:
            typeof event.output === "string" ? event.output : JSON.stringify(event.output ?? {}),
          is_error: Boolean(event.is_error),
        },
      });
      return;
    case "usage": {
      const inputTokens =
        typeof event.input_tokens === "number" ? Math.max(event.input_tokens, 0) : 0;
      const outputTokens =
        typeof event.output_tokens === "number" ? Math.max(event.output_tokens, 0) : 0;
      const cacheReadTokens =
        typeof event.cache_read_tokens === "number" ? Math.max(event.cache_read_tokens, 0) : 0;
      const nextSpent =
        typeof event.cost_usd === "number" && Number.isFinite(event.cost_usd)
          ? runtime.spentUsd + Math.max(event.cost_usd, 0)
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
      return;
    }
    case "error":
      if (typeof event.message === "string") {
        throw new Error(event.message);
      }
      return;
    default:
      if (typeof event.content === "string" && event.content.length > 0) {
        streamer.send({
          task_id: req.task_id,
          session_id: req.session_id,
          timestamp_ms: now(),
          type: "output",
          data: {
            content: event.content,
            content_type: "text",
            turn_number: runtime.turnNumber,
          },
        });
      }
  }
}

async function* spawnCommandRuntime(params: {
  runtime: CommandRuntimeKey;
  command: string;
  req: ExecuteRequest;
  systemPrompt: string;
  abortSignal: AbortSignal;
}): AsyncGenerator<UnknownRecord, void> {
  const proc = Bun.spawn({
    cmd: [params.command],
    cwd: params.req.worktree_path,
    env: {
      ...process.env,
      AGENTFORGE_RUNTIME: params.runtime,
      AGENTFORGE_MODEL: params.req.model ?? "",
      AGENTFORGE_PERMISSION_MODE: params.req.permission_mode,
    },
    stdin: "pipe",
    stdout: "pipe",
    stderr: "pipe",
  });

  const onAbort = () => {
    try {
      proc.kill();
    } catch {
      // Best-effort cancellation only.
    }
  };
  params.abortSignal.addEventListener("abort", onAbort, { once: true });

  try {
    const input = JSON.stringify({
      task_id: params.req.task_id,
      session_id: params.req.session_id,
      prompt: params.req.prompt,
      system_prompt: params.systemPrompt,
      worktree_path: params.req.worktree_path,
      branch_name: params.req.branch_name,
      model: params.req.model,
      allowed_tools: params.req.allowed_tools,
      permission_mode: params.req.permission_mode,
      budget_usd: params.req.budget_usd,
      max_turns: params.req.max_turns,
    });
    proc.stdin.write(`${input}\n`);
    proc.stdin.end();

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
          // Fall through and emit raw text as assistant output.
        }

        yield {
          type: "assistant_text",
          content: line,
        };
      }
    }

    const exitCode = await proc.exited;
    const stderr = proc.stderr ? await readToString(proc.stderr) : "";
    if (exitCode !== 0 && !params.abortSignal.aborted) {
      throw new Error(stderr.trim() || `${params.runtime} runtime exited with code ${exitCode}`);
    }
  } finally {
    params.abortSignal.removeEventListener("abort", onAbort);
  }
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
