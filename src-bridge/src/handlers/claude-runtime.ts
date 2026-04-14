import {
  accumulateCostAccounting,
  serializeCostAccounting,
  type CostAccountingComponentInput,
} from "../cost/accounting.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { SessionManager } from "../session/manager.js";
import type {
  ClaudeContinuityState,
  ExecuteRequest,
  RuntimeContinuityState,
  SessionSnapshot,
} from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";
import type { MCPClientHub } from "../mcp/client-hub.js";
import type { PluginRecord } from "../plugins/types.js";
import {
  buildFilterPipeline,
  applyFilters,
  type OutputFilter,
} from "../filters/pipeline.js";
import type { Options } from "@anthropic-ai/claude-agent-sdk";
import { HookCallbackManager } from "../runtime/hook-callback-manager.js";
import { emitBudgetAlertIfNeeded } from "./budget-events.js";

type UnknownRecord = Record<string, unknown>;
type EventSink = Pick<EventStreamer, "send">;
interface UsageInfo {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_input_tokens?: number;
  cache_creation_input_tokens?: number;
}
type QueryRunnerResult = AsyncIterable<UnknownRecord> & {
  interrupt?: () => Promise<void>;
  setModel?: (model?: string) => Promise<void>;
  setMaxThinkingTokens?: (maxThinkingTokens: number | null) => Promise<void>;
  rewindFiles?: (
    userMessageId: string,
    options?: { dryRun?: boolean },
  ) => Promise<{ canRewind: boolean; error?: string }>;
  mcpServerStatus?: () => Promise<unknown>;
};
type ForkSessionRunner = (
  sessionId: string,
  options?: {
    dir?: string;
    upToMessageId?: string;
    title?: string;
  },
) => Promise<{ sessionId: string }>;

export type QueryRunner = (params: {
  prompt: string;
  options?: Record<string, unknown>;
}) => QueryRunnerResult;

export interface ClaudeRuntimeDeps {
  queryRunner?: QueryRunner;
  sessionManager?: SessionManager;
  now?: () => number;
  mcpHub?: MCPClientHub;
  activePlugins?: PluginRecord[];
  continuity?: RuntimeContinuityState;
  hookCallbackManager?: HookCallbackManager;
  forkSessionRunner?: ForkSessionRunner;
}

/**
 * Build MCP server configs from active plugin records for the Claude Agent SDK.
 * Converts plugin specs to the SDK's McpServerConfig format.
 */
export function buildMcpServersOption(
  plugins: PluginRecord[],
): Record<string, unknown> | undefined {
  if (plugins.length === 0) return undefined;

  const servers: Record<string, unknown> = {};
  for (const plugin of plugins) {
    const id = plugin.metadata.id;
    if (plugin.spec.transport === "http" && plugin.spec.url) {
      servers[id] = { type: "http", url: plugin.spec.url };
    } else if (plugin.spec.command) {
      servers[id] = {
        command: plugin.spec.command,
        args: plugin.spec.args,
        env: plugin.spec.env,
      };
    }
  }
  return Object.keys(servers).length > 0 ? servers : undefined;
}

export function buildClaudeQueryOptions(
  req: ExecuteRequest,
  systemPrompt: string,
  runtime: AgentRuntime,
  activePlugins?: PluginRecord[],
  continuity?: RuntimeContinuityState,
): Options & Record<string, unknown> {
  const options: Options & Record<string, unknown> = {
    abortController: runtime.abortController,
    allowedTools: req.allowed_tools.length > 0 ? req.allowed_tools : undefined,
    cwd: req.worktree_path,
    maxBudgetUsd: req.budget_usd,
    maxTurns: req.max_turns,
    model: req.model,
    permissionMode: req.permission_mode as Options["permissionMode"],
    systemPrompt,
  };

  if (req.agents) {
    options.agents = req.agents;
  }
  if (req.thinking_config?.enabled && req.thinking_config.budget_tokens) {
    options.maxThinkingTokens = req.thinking_config.budget_tokens;
  }
  if (req.file_checkpointing) {
    options.enableFileCheckpointing = true;
  }
  if (req.output_schema) {
    options.outputFormat = req.output_schema;
  }
  if (req.include_partial_messages) {
    options.includePartialMessages = true;
  }
  if (req.disallowed_tools?.length) {
    options.disallowedTools = req.disallowed_tools;
  }
  if (req.fallback_model) {
    options.fallbackModel = req.fallback_model;
  }
  if (req.additional_directories?.length) {
    options.additionalDirectories = req.additional_directories;
  }
  if (req.env) {
    options.env = {
      ...process.env,
      ...req.env,
    };
  }

  if (req.permission_mode === "bypassPermissions") {
    options.allowDangerouslySkipPermissions = true;
  }

  // Inject MCP server configs so the Claude Agent SDK manages tool connections
  if (activePlugins && activePlugins.length > 0) {
    const mcpServers = buildMcpServersOption(activePlugins);
    if (mcpServers) {
      (options as Record<string, unknown>).mcpServers = mcpServers;
    }
  }

  if (
    continuity?.runtime === "claude_code" &&
    continuity.resume_ready &&
    continuity.session_handle
  ) {
    options.resume = continuity.resume_token ?? continuity.session_handle;
    if (continuity.checkpoint_id) {
      options.resumeSessionAt = continuity.checkpoint_id;
    }
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
  const queryRunner = deps.queryRunner ?? (await loadDefaultQueryRunner());
  const now = deps.now ?? Date.now;
  const options = buildClaudeQueryOptions(
    req,
    systemPrompt,
    runtime,
    deps.activePlugins,
    deps.continuity,
  );
  attachClaudeCallbacks(options, streamer, req, deps);

  // Build output filters from role config
  const outputFilters = buildFilterPipeline(
    req.role_config?.output_filters ?? [],
  );

  // Periodic cost reporting timer (every 5s)
  const costReportInterval = setInterval(() => {
    if (runtime.spentUsd > 0 && runtime.costAccounting) {
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "cost_update",
        data: {
          session_id: req.session_id,
          input_tokens: runtime.costAccounting.inputTokens,
          output_tokens: runtime.costAccounting.outputTokens,
          cache_read_tokens: runtime.costAccounting.cacheReadTokens,
          cache_creation_tokens: runtime.costAccounting.cacheCreationTokens,
          cost_usd: runtime.spentUsd,
          budget_remaining_usd: Math.max(req.budget_usd - runtime.spentUsd, 0),
          turn_number: runtime.turnNumber,
          periodic: true,
          cost_accounting: serializeCostAccounting(runtime.costAccounting),
        },
      });
    }
  }, 5000);

  try {
    const query = queryRunner({
      prompt: req.prompt,
      options,
    });
    runtime.claudeQuery = hasClaudeQueryControls(query) ? query : null;

    for await (const message of query) {
      runtime.lastActivity = now();
      runtime.continuity = enrichClaudeContinuity(
        extractClaudeContinuity(
          message,
          runtime.lastActivity,
          runtime.continuity,
        ),
        runtime,
      );

      emitAssistantBlocks(runtime, streamer, message, req, now, outputFilters);
      emitToolResult(streamer, message, req, now);
      emitPartialAssistantMessage(streamer, message, req, now);
      emitRateLimit(streamer, message, req, now);
      emitToolProgress(streamer, message, req, now);
      emitCompactBoundary(streamer, message, req, now);
      captureStructuredOutput(runtime, message);
      emitUsage(runtime, streamer, message, req, now);

      if (runtime.spentUsd >= req.budget_usd) {
        runtime.abortController.abort("budget_exceeded");
        throw new Error(`budget exceeded for task ${req.task_id}`);
      }
    }
  } finally {
    if (runtime.claudeQuery && runtime.status !== "running") {
      runtime.claudeQuery = null;
    }
    clearInterval(costReportInterval);
  }
}

export function extractClaudeContinuity(
  message: UnknownRecord,
  capturedAt: number,
  previous: RuntimeContinuityState | null = null,
): ClaudeContinuityState | null {
  const previousClaude = previous?.runtime === "claude_code" ? previous : null;
  const sessionHandle =
    typeof message.session_id === "string" && message.session_id.length > 0
      ? message.session_id
      : previousClaude?.session_handle;

  if (!sessionHandle) {
    return previousClaude;
  }

  const checkpointId =
    message.type === "assistant" &&
    typeof message.uuid === "string" &&
    message.uuid.length > 0
      ? message.uuid
      : previousClaude?.checkpoint_id;

  return {
    runtime: "claude_code",
    resume_ready: true,
    captured_at: capturedAt,
    session_handle: sessionHandle,
    checkpoint_id: checkpointId,
    resume_token: sessionHandle,
  };
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

async function loadDefaultQueryRunner(): Promise<QueryRunner> {
  const sdk = await import("@anthropic-ai/claude-agent-sdk");
  return sdk.query as unknown as QueryRunner;
}

function attachClaudeCallbacks(
  options: Options & Record<string, unknown>,
  streamer: EventSink,
  req: ExecuteRequest,
  deps: ClaudeRuntimeDeps,
): void {
  if (req.hooks_config?.hooks.length && deps.hookCallbackManager) {
    const hooks = req.hooks_config.hooks.reduce<
      Record<string, Array<Record<string, unknown>>>
    >((acc, hookDefinition) => {
      const eventName = hookDefinition.hook;
      const matcher = {
        matcher:
          typeof hookDefinition.matcher === "string"
            ? hookDefinition.matcher
            : hookDefinition.matcher
              ? JSON.stringify(hookDefinition.matcher)
              : undefined,
        timeout: toHookTimeoutSeconds(req),
        hooks: [
          async (input: Record<string, unknown>) => {
            return requestHookCallback(deps.hookCallbackManager!, req, {
              hook: eventName,
              task_id: req.task_id,
              session_id: req.session_id,
              ...normalizeHookInput(input),
            });
          },
        ],
      };
      if (!acc[eventName]) {
        acc[eventName] = [];
      }
      acc[eventName]?.push(matcher);
      return acc;
    }, {});
    options.hooks = hooks;
  }

  if (
    req.tool_permission_callback &&
    resolveHookCallbackUrl(req) &&
    deps.hookCallbackManager
  ) {
    options.canUseTool = async (
      toolName: string,
      input: Record<string, unknown>,
      callbackOptions: Record<string, unknown>,
    ) => {
      const result = await requestPermissionCallback(
        deps.hookCallbackManager!,
        req,
        streamer,
        {
          callback_type: "tool_permission",
          tool_name: toolName,
          tool_input: input,
          context: callbackOptions,
        },
      );
      return result?.decision === "deny"
        ? {
            behavior: "deny",
            message: result.reason ?? "Tool usage denied",
          }
        : {
            behavior: "allow",
          };
    };
  }

  if (resolveHookCallbackUrl(req) && deps.hookCallbackManager) {
    options.onElicitation = async (
      requestPayload: Record<string, unknown>,
      callbackOptions: Record<string, unknown>,
    ) => {
      const result = await requestPermissionCallback(
        deps.hookCallbackManager!,
        req,
        streamer,
        {
          callback_type: "elicitation",
          mcp_server_id: requestPayload.serverName,
          elicitation_type: requestPayload.mode,
          message: requestPayload.message,
          url: requestPayload.url,
          fields: requestPayload.requestedSchema
            ? [requestPayload.requestedSchema]
            : undefined,
          context: callbackOptions,
        },
      );
      return result?.decision === "deny"
        ? { action: "decline" }
        : { action: "accept" };
    };
  }
}

async function requestPermissionCallback(
  manager: HookCallbackManager,
  req: ExecuteRequest,
  streamer: EventSink,
  payload: Record<string, unknown>,
): Promise<{ decision?: string; reason?: string } | null> {
  try {
    const pending = await manager.register({
      callbackUrl: resolveHookCallbackUrl(req),
      payload,
      timeoutMs: req.hook_timeout_ms ?? req.hooks_config?.timeout_ms ?? 5000,
    });

    streamer.send({
      task_id: req.task_id,
      session_id: req.session_id,
      timestamp_ms: Date.now(),
      type: "permission_request",
      data: {
        request_id: pending.requestId,
        tool_name: payload.tool_name,
        elicitation_type: payload.elicitation_type,
        fields: payload.fields,
        mcp_server_id: payload.mcp_server_id,
        context: payload.context,
      },
    });

    return (await pending.response) as { decision?: string; reason?: string };
  } catch {
    return {
      decision: "allow",
    };
  }
}

async function requestHookCallback(
  manager: HookCallbackManager,
  req: ExecuteRequest,
  payload: Record<string, unknown>,
): Promise<Record<string, unknown>> {
  try {
    const pending = await manager.register({
      callbackUrl: resolveHookCallbackUrl(req),
      payload,
      timeoutMs: req.hook_timeout_ms ?? req.hooks_config?.timeout_ms ?? 5000,
    });
    return (await pending.response) as Record<string, unknown>;
  } catch {
    return {
      continue: true,
    };
  }
}

function normalizeHookInput(
  input: Record<string, unknown>,
): Record<string, unknown> {
  return {
    tool_name: input.tool_name,
    tool_input: input.tool_input,
    tool_use_id: input.tool_use_id,
    mcp_server_id: input.mcp_server_name,
    elicitation_type: input.mode,
    fields: input.requested_schema ? [input.requested_schema] : undefined,
    url: input.url,
    message: input.message,
  };
}

function toHookTimeoutSeconds(req: ExecuteRequest): number | undefined {
  const timeoutMs = req.hook_timeout_ms ?? req.hooks_config?.timeout_ms;
  if (!timeoutMs || timeoutMs <= 0) {
    return undefined;
  }
  return Math.ceil(timeoutMs / 1000);
}

function resolveHookCallbackUrl(req: ExecuteRequest): string {
  return req.hook_callback_url ?? req.hooks_config?.callback_url ?? "";
}

function hasClaudeQueryControls(query: QueryRunnerResult): boolean {
  return (
    typeof query.interrupt === "function" ||
    typeof query.setModel === "function" ||
    typeof query.setMaxThinkingTokens === "function" ||
    typeof query.mcpServerStatus === "function" ||
    typeof query.rewindFiles === "function"
  );
}

function enrichClaudeContinuity(
  continuity: ClaudeContinuityState | null,
  runtime: AgentRuntime,
): ClaudeContinuityState | null {
  if (!continuity) {
    return null;
  }

  return {
    ...continuity,
    query_ref: runtime.taskId,
    fork_available: true,
  };
}

function emitAssistantBlocks(
  runtime: AgentRuntime,
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
  outputFilters: OutputFilter[] = [],
): void {
  if (message.type !== "assistant") return;

  const contentBlocks = getContentBlocks(message);
  if (!contentBlocks) return;

  for (const block of contentBlocks) {
    if (block.type === "text" && typeof block.text === "string") {
      const content =
        outputFilters.length > 0
          ? applyFilters(block.text, outputFilters)
          : block.text;
      streamer.send({
        task_id: req.task_id,
        session_id: req.session_id,
        timestamp_ms: now(),
        type: "output",
        data: {
          content,
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

function emitPartialAssistantMessage(
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (message.type !== "stream_event" || !isRecord(message.event)) {
    return;
  }

  const content =
    typeof message.event.text === "string"
      ? message.event.text
      : isRecord(message.event.delta) &&
          typeof message.event.delta.text === "string"
        ? message.event.delta.text
        : isRecord(message.event.content_block) &&
            typeof message.event.content_block.text === "string"
          ? message.event.content_block.text
          : null;
  if (!content) {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "partial_message",
    data: {
      content,
      is_complete: false,
    },
  });
}

function emitRateLimit(
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (
    message.type !== "rate_limit_event" ||
    !isRecord(message.rate_limit_info)
  ) {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "rate_limit",
    data: {
      utilization:
        typeof message.rate_limit_info.utilization === "number"
          ? message.rate_limit_info.utilization
          : undefined,
      reset_at:
        typeof message.rate_limit_info.resetsAt === "number"
          ? message.rate_limit_info.resetsAt
          : undefined,
      status:
        typeof message.rate_limit_info.status === "string"
          ? message.rate_limit_info.status
          : undefined,
    },
  });
}

function emitToolProgress(
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (message.type !== "tool_progress") {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "progress",
    data: {
      tool_name:
        typeof message.tool_name === "string" ? message.tool_name : undefined,
      progress_text:
        typeof message.tool_name === "string" &&
        typeof message.elapsed_time_seconds === "number"
          ? `${message.tool_name} running (${message.elapsed_time_seconds}s)`
          : undefined,
      elapsed_time_seconds:
        typeof message.elapsed_time_seconds === "number"
          ? message.elapsed_time_seconds
          : undefined,
      tool_use_id:
        typeof message.tool_use_id === "string"
          ? message.tool_use_id
          : undefined,
      task_id:
        typeof message.task_id === "string" ? message.task_id : undefined,
    },
  });
}

function emitCompactBoundary(
  streamer: EventSink,
  message: UnknownRecord,
  req: ExecuteRequest,
  now: () => number,
): void {
  if (message.type !== "system" || message.subtype !== "compact_boundary") {
    return;
  }

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "status_change",
    data: {
      old_status: "running",
      new_status: "running",
      reason: "compact_boundary",
      compact_metadata: message.compact_metadata,
    },
  });
}

function captureStructuredOutput(
  runtime: AgentRuntime,
  message: UnknownRecord,
): void {
  if (message.type !== "result" || !isRecord(message.structured_output)) {
    return;
  }

  runtime.structuredOutput = message.structured_output;
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
  const source =
    typeof message.total_cost_usd === "number" && Number.isFinite(message.total_cost_usd)
      ? "anthropic_result_total"
      : "anthropic_api_pricing";
  const components = extractModelUsageComponents(message);

  const reportedTotal =
    typeof message.total_cost_usd === "number" &&
    Number.isFinite(message.total_cost_usd)
      ? message.total_cost_usd
      : undefined;
  const snapshot = accumulateCostAccounting({
    previous: runtime.costAccounting,
    runtime: req.runtime ?? "claude_code",
    provider: req.provider ?? "anthropic",
    requestedModel: req.model ?? "claude-sonnet-4-5",
    usageDelta: {
      inputTokens: usage.input_tokens ?? 0,
      outputTokens: usage.output_tokens ?? 0,
      cacheReadTokens: usage.cache_read_input_tokens ?? 0,
      cacheCreationTokens: usage.cache_creation_input_tokens ?? 0,
    },
    authoritativeTotalCostUsd: reportedTotal,
    source,
    components,
  });
  runtime.applyCostAccounting(snapshot);

  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "cost_update",
    data: {
      session_id: req.session_id,
      input_tokens: usage.input_tokens ?? 0,
      output_tokens: usage.output_tokens ?? 0,
      cache_read_tokens: usage.cache_read_input_tokens ?? 0,
      cache_creation_tokens: usage.cache_creation_input_tokens ?? 0,
      cost_usd: runtime.spentUsd,
      budget_remaining_usd: Math.max(req.budget_usd - runtime.spentUsd, 0),
      turn_number: runtime.turnNumber,
      cost_accounting: serializeCostAccounting(snapshot),
    },
  });
  emitBudgetAlertIfNeeded(runtime, streamer, req, now);
}

function extractUsage(message: UnknownRecord): UsageInfo | null {
  const sourceUsage = isRecord(message.usage)
    ? message.usage
    : isRecord(message.message) && isRecord(message.message.usage)
      ? message.message.usage
      : null;
  if (!sourceUsage) return null;

  return {
    input_tokens:
      typeof sourceUsage.input_tokens === "number"
        ? sourceUsage.input_tokens
        : undefined,
    output_tokens:
      typeof sourceUsage.output_tokens === "number"
        ? sourceUsage.output_tokens
        : undefined,
    cache_read_input_tokens:
      typeof sourceUsage.cache_read_input_tokens === "number"
        ? sourceUsage.cache_read_input_tokens
        : undefined,
    cache_creation_input_tokens:
      typeof sourceUsage.cache_creation_input_tokens === "number"
        ? sourceUsage.cache_creation_input_tokens
        : undefined,
  };
}

function extractModelUsageComponents(
  message: UnknownRecord,
): CostAccountingComponentInput[] {
  if (!isRecord(message.modelUsage)) {
    return [];
  }

  return Object.entries(message.modelUsage).flatMap(([model, usage]) => {
    if (!isRecord(usage)) {
      return [];
    }

    return [
      {
        model,
        inputTokens:
          typeof usage.inputTokens === "number" ? usage.inputTokens : undefined,
        outputTokens:
          typeof usage.outputTokens === "number" ? usage.outputTokens : undefined,
        cacheReadTokens:
          typeof usage.cacheReadInputTokens === "number"
            ? usage.cacheReadInputTokens
            : undefined,
        cacheCreationTokens:
          typeof usage.cacheCreationInputTokens === "number"
            ? usage.cacheCreationInputTokens
            : undefined,
        costUsd: typeof usage.costUSD === "number" ? usage.costUSD : undefined,
        source: "anthropic_model_usage",
      },
    ];
  });
}

function getContentBlocks(message: UnknownRecord): Array<UnknownRecord> | null {
  if (!isRecord(message.message) || !Array.isArray(message.message.content))
    return null;

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
