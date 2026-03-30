import { existsSync, readFileSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import { basename, join } from "node:path";
import {
  streamClaudeRuntime,
  type ClaudeRuntimeDeps,
} from "../handlers/claude-runtime.js";
import {
  type CommandRuntimeRunner,
} from "../handlers/command-runtime.js";
import {
  getDefaultCodexAuthStatus,
  streamCodexRuntime,
  type CodexAuthStatusProvider,
  type CodexRuntimeRunner,
} from "../handlers/codex-runtime.js";
import {
  streamOpenCodeRuntime,
  type OpenCodeEventRunner,
} from "../handlers/opencode-runtime.js";
import {
  createOpenCodeTransport,
  type OpenCodeTransport,
} from "../opencode/transport.js";
import type { AgentRuntime } from "./agent-runtime.js";
import type {
  ExecuteRequest,
  AgentRuntimeKey,
  RuntimeCatalog,
  RuntimeCatalogEntry,
  RuntimeDiagnostic,
  RuntimeContinuityState,
} from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";
import {
  UnsupportedOperationError,
  type RuntimeOperationName,
} from "./errors.js";

type EventSink = Pick<EventStreamer, "send">;

export interface RuntimeForkParams {
  message_id?: string;
}

export interface RuntimeForkResult {
  continuity: RuntimeContinuityState;
}

export interface RuntimeRollbackParams {
  checkpoint_id?: string;
  turns?: number;
}

export interface RuntimeRevertParams {
  action: "revert" | "unrevert";
  message_id?: string;
}

export interface RuntimeCommandParams {
  command: string;
  arguments?: string;
}

export interface RuntimeSetModelParams {
  model: string;
}

export interface RuntimeAdvancedOperations {
  fork?: (runtime: AgentRuntime, params: RuntimeForkParams) => Promise<RuntimeForkResult>;
  rollback?: (runtime: AgentRuntime, params: RuntimeRollbackParams) => Promise<void>;
  revert?: (runtime: AgentRuntime, params: RuntimeRevertParams) => Promise<void>;
  getMessages?: (runtime: AgentRuntime) => Promise<unknown>;
  getDiff?: (runtime: AgentRuntime) => Promise<unknown>;
  executeCommand?: (runtime: AgentRuntime, params: RuntimeCommandParams) => Promise<unknown>;
  interrupt?: (runtime: AgentRuntime) => Promise<void>;
  setModel?: (runtime: AgentRuntime, params: RuntimeSetModelParams) => Promise<void>;
}

type RequiredRuntimeAdvancedOperations = {
  [K in keyof RuntimeAdvancedOperations]-?: NonNullable<RuntimeAdvancedOperations[K]>;
};

interface CodexForkProcess {
  stdout?: ReadableStream<Uint8Array> | null;
  stderr?: ReadableStream<Uint8Array> | null;
  kill(): void;
  exited: Promise<number>;
}

interface CodexRolloutMeta {
  threadId?: string;
  forkedFromId?: string;
}

export interface DefaultCodexForkRunnerDeps {
  getSessionsRoot?: () => string;
  listRolloutFiles?: (root: string) => string[];
  readRolloutMeta?: (filePath: string) => CodexRolloutMeta | null;
  sleep?: (ms: number) => Promise<void>;
  timeoutMs?: number;
  spawn?: (params: {
    cmd: string[];
    cwd?: string;
    stdin: "ignore";
    stdout: "pipe";
    stderr: "pipe";
  }) => CodexForkProcess;
}

export class UnknownRuntimeError extends Error {}
export class RuntimeConfigurationError extends Error {}
export class UnsupportedRuntimeProviderError extends Error {}

interface RuntimeAdapter {
  key: AgentRuntimeKey;
  label: string;
  defaultProvider: string;
  compatibleProviders: string[];
  defaultModel?: string;
  getDiagnostics(): Promise<RuntimeDiagnostic[]>;
  getCatalogDetails?(): Promise<Partial<RuntimeCatalogEntry>>;
  ensureAvailable(): Promise<void>;
  execute(
    runtime: AgentRuntime,
    streamer: EventSink,
    req: ExecuteRequest,
    systemPrompt: string,
  ): Promise<void>;
  fork(runtime: AgentRuntime, params: RuntimeForkParams): Promise<RuntimeForkResult>;
  rollback(runtime: AgentRuntime, params: RuntimeRollbackParams): Promise<void>;
  revert(runtime: AgentRuntime, params: RuntimeRevertParams): Promise<void>;
  getMessages(runtime: AgentRuntime): Promise<unknown>;
  getDiff(runtime: AgentRuntime): Promise<unknown>;
  executeCommand(runtime: AgentRuntime, params: RuntimeCommandParams): Promise<unknown>;
  interrupt(runtime: AgentRuntime): Promise<void>;
  setModel(runtime: AgentRuntime, params: RuntimeSetModelParams): Promise<void>;
}

export interface AgentRuntimeRegistryOptions extends ClaudeRuntimeDeps {
  commandRuntimeRunner?: CommandRuntimeRunner;
  codexRuntimeRunner?: CodexRuntimeRunner;
  defaultRuntime?: AgentRuntimeKey;
  executableLookup?: (command: string) => string | null;
  envLookup?: (name: string) => string | undefined;
  opencodeTransport?: OpenCodeTransport;
  codexAuthStatusProvider?: CodexAuthStatusProvider;
  continuity?: RuntimeContinuityState;
  opencodeEventRunner?: OpenCodeEventRunner;
  advancedOperations?: Partial<Record<AgentRuntimeKey, RuntimeAdvancedOperations>>;
  forkSessionRunner?: (
    sessionId: string,
    options?: {
      dir?: string;
      upToMessageId?: string;
      title?: string;
    },
  ) => Promise<{ sessionId: string }>;
  codexForkRunner?: (params: {
    command: string;
    threadId: string;
    cwd?: string;
  }) => Promise<{ threadId: string }>;
}

export class AgentRuntimeRegistry {
  constructor(
    private readonly adapters: Record<AgentRuntimeKey, RuntimeAdapter>,
    private readonly defaultRuntime: AgentRuntimeKey,
  ) {}

  async resolveExecute(
    req: ExecuteRequest,
  ): Promise<{ adapter: RuntimeAdapter; request: ExecuteRequest }> {
    const runtimeKey = resolveRuntimeKey(req, this.defaultRuntime);
    const adapter = this.adapters[runtimeKey];
    if (!adapter) {
      throw new UnknownRuntimeError(`Unknown runtime: ${runtimeKey}`);
    }

    const provider = normalizeProvider(req.provider) || adapter.defaultProvider;
    validateRuntimeProvider(runtimeKey, provider, adapter.compatibleProviders);
    await adapter.ensureAvailable();

    return {
      adapter,
      request: {
        ...req,
        runtime: runtimeKey,
        provider,
        model: req.model ?? adapter.defaultModel,
      },
    };
  }

  async getCatalog(): Promise<RuntimeCatalog> {
    const runtimes = await Promise.all(
      Object.values(this.adapters).map(async (adapter) => {
        const diagnostics = await adapter.getDiagnostics();
        return {
          key: adapter.key,
          label: adapter.label,
          defaultProvider: adapter.defaultProvider,
          compatibleProviders: [...adapter.compatibleProviders],
          defaultModel: adapter.defaultModel,
          available: !diagnostics.some((diagnostic) => diagnostic.blocking),
          diagnostics,
          supportedFeatures: getSupportedFeatures(adapter.key),
          ...(adapter.getCatalogDetails ? await adapter.getCatalogDetails() : {}),
        } satisfies RuntimeCatalogEntry;
      }),
    );

    return {
      defaultRuntime: this.defaultRuntime,
      runtimes,
    };
  }

  async fork(
    runtime: AgentRuntime,
    params: RuntimeForkParams,
  ): Promise<RuntimeForkResult> {
    return this.getAdapterForRuntime(runtime).fork(runtime, params);
  }

  async rollback(runtime: AgentRuntime, params: RuntimeRollbackParams): Promise<void> {
    await this.getAdapterForRuntime(runtime).rollback(runtime, params);
  }

  async revert(runtime: AgentRuntime, params: RuntimeRevertParams): Promise<void> {
    await this.getAdapterForRuntime(runtime).revert(runtime, params);
  }

  async getMessages(runtime: AgentRuntime): Promise<unknown> {
    return this.getAdapterForRuntime(runtime).getMessages(runtime);
  }

  async getDiff(runtime: AgentRuntime): Promise<unknown> {
    return this.getAdapterForRuntime(runtime).getDiff(runtime);
  }

  async executeCommand(
    runtime: AgentRuntime,
    params: RuntimeCommandParams,
  ): Promise<unknown> {
    return this.getAdapterForRuntime(runtime).executeCommand(runtime, params);
  }

  async interrupt(runtime: AgentRuntime): Promise<void> {
    await this.getAdapterForRuntime(runtime).interrupt(runtime);
  }

  async setModel(runtime: AgentRuntime, params: RuntimeSetModelParams): Promise<void> {
    await this.getAdapterForRuntime(runtime).setModel(runtime, params);
  }

  private getAdapterForRuntime(runtime: AgentRuntime): RuntimeAdapter {
    const runtimeKey =
      runtime.request?.runtime ?? runtime.continuity?.runtime ?? this.defaultRuntime;
    const adapter = this.adapters[runtimeKey];
    if (!adapter) {
      throw new UnknownRuntimeError(`Unknown runtime: ${runtimeKey}`);
    }
    return adapter;
  }
}

export function createRuntimeRegistry(
  options: AgentRuntimeRegistryOptions = {},
): AgentRuntimeRegistry {
  const executableLookup = options.executableLookup ?? defaultExecutableLookup;
  const envLookup = options.envLookup ?? ((name: string) => process.env[name]);
  const codexCommand = readEnvConfig(envLookup, "CODEX_RUNTIME_COMMAND") || "codex";
  const opencodeTransport =
    options.opencodeTransport ??
    createOpenCodeTransport({
      envLookup,
    });
  const codexAuthStatusProvider =
    options.codexAuthStatusProvider ?? (() => getDefaultCodexAuthStatus(codexCommand));

  const adapters: Record<AgentRuntimeKey, RuntimeAdapter> = {
    claude_code: {
      key: "claude_code",
      label: "Claude Code",
      defaultProvider: "anthropic",
      compatibleProviders: ["anthropic"],
      defaultModel: readEnvConfig(envLookup, "CLAUDE_CODE_RUNTIME_MODEL"),
      async getDiagnostics() {
        if (options.queryRunner) {
          return [];
        }
        const diagnostics: RuntimeDiagnostic[] = [];
        const apiKey = envLookup("ANTHROPIC_API_KEY")?.trim();
        if (!apiKey) {
          diagnostics.push({
            code: "missing_credentials",
            message:
              "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
            blocking: true,
          });
        }
        return diagnostics;
      },
      async ensureAvailable() {
        assertDiagnosticsAvailable("claude_code", await this.getDiagnostics());
      },
      async execute(runtime, streamer, req, systemPrompt) {
        await streamClaudeRuntime(runtime, streamer, req, systemPrompt, {
          continuity: options.continuity,
          queryRunner: options.queryRunner,
          hookCallbackManager: options.hookCallbackManager,
          forkSessionRunner: options.forkSessionRunner,
          now: options.now,
        });
      },
      ...createClaudeAdvancedOperations(options),
    },
    codex: createCodexAdapter({
      executableLookup,
      authStatusProvider: codexAuthStatusProvider,
      codexRuntimeRunner: options.codexRuntimeRunner,
      defaultCommand: codexCommand,
      defaultModel: readEnvConfig(envLookup, "CODEX_RUNTIME_MODEL"),
      now: options.now,
      activePlugins: options.activePlugins,
      advancedOperations: options.advancedOperations?.codex,
      codexForkRunner: options.codexForkRunner,
    }),
    opencode: createOpenCodeReadinessAdapter({
      transport: opencodeTransport,
      eventRunner: options.opencodeEventRunner,
      defaultModel: readEnvConfig(envLookup, "OPENCODE_RUNTIME_MODEL"),
      now: options.now,
      advancedOperations: options.advancedOperations?.opencode,
    }),
  };

  return new AgentRuntimeRegistry(adapters, options.defaultRuntime ?? "claude_code");
}

function createCodexAdapter(options: {
  executableLookup: (command: string) => string | null;
  authStatusProvider: CodexAuthStatusProvider;
  codexRuntimeRunner?: CodexRuntimeRunner;
  defaultCommand: string;
  defaultModel?: string;
  now?: () => number;
  activePlugins?: ClaudeRuntimeDeps["activePlugins"];
  advancedOperations?: RuntimeAdvancedOperations;
  codexForkRunner?: AgentRuntimeRegistryOptions["codexForkRunner"];
}): RuntimeAdapter {
  return {
    key: "codex",
    label: "Codex",
    defaultProvider: "openai",
    compatibleProviders: ["openai", "codex"],
    defaultModel: options.defaultModel,
    async getDiagnostics() {
      const diagnostics: RuntimeDiagnostic[] = [];
      const resolved = options.executableLookup(options.defaultCommand);
      if (!resolved) {
        diagnostics.push({
          code: "missing_executable",
          message: "Executable not found for runtime codex",
          blocking: true,
        });
        return diagnostics;
      }

      const authStatus = options.authStatusProvider();
      if (!authStatus.authenticated) {
        diagnostics.push({
          code: "missing_credentials",
          message: authStatus.message || "Codex CLI authentication is unavailable",
          blocking: true,
        });
      }

      return diagnostics;
    },
    async ensureAvailable() {
      assertDiagnosticsAvailable("codex", await this.getDiagnostics());
    },
    async execute(runtime, streamer, req, systemPrompt) {
      await streamCodexRuntime(runtime, streamer, req, systemPrompt, {
        command: options.defaultCommand,
        codexRuntimeRunner: options.codexRuntimeRunner,
        now: options.now,
        activePlugins: options.activePlugins,
      });
    },
    ...createCodexAdvancedOperations(options),
  };
}

function createOpenCodeReadinessAdapter(options: {
  transport: OpenCodeTransport;
  eventRunner?: OpenCodeEventRunner;
  defaultModel?: string;
  now?: () => number;
  advancedOperations?: RuntimeAdvancedOperations;
}): RuntimeAdapter {
  return {
    key: "opencode",
    label: "OpenCode",
    defaultProvider: "opencode",
    compatibleProviders: ["opencode"],
    defaultModel: options.defaultModel,
    async getDiagnostics() {
      const readiness = await options.transport.checkReadiness({
        provider: "opencode",
        model: options.defaultModel,
      });
      return readiness.diagnostics;
    },
    async getCatalogDetails() {
      try {
        return {
          agents: await options.transport.getAgents(),
          skills: await options.transport.getSkills(),
        };
      } catch {
        return {};
      }
    },
    async ensureAvailable() {
      assertDiagnosticsAvailable("opencode", await this.getDiagnostics());
    },
    async execute(runtime, streamer, req, systemPrompt) {
      await streamOpenCodeRuntime(runtime, streamer, req, systemPrompt, {
        transport: options.transport,
        eventRunner: options.eventRunner,
        now: options.now,
      });
    },
    ...createOpenCodeAdvancedOperations(options),
  };
}

function createClaudeAdvancedOperations(
  options: AgentRuntimeRegistryOptions,
): RequiredRuntimeAdvancedOperations {
  const overrides = options.advancedOperations?.claude_code;
  return {
    fork:
      overrides?.fork ??
      (async (runtime, params) => {
        const sessionHandle =
          runtime.continuity?.runtime === "claude_code"
            ? runtime.continuity.session_handle
            : undefined;
        if (!sessionHandle) {
          throw new UnsupportedOperationError("fork", "claude_code");
        }

        const runner = options.forkSessionRunner ?? defaultForkSessionRunner;
        const result = await runner(sessionHandle, {
          dir: runtime.request?.worktree_path,
          upToMessageId: params.message_id,
        });
        return {
          continuity: {
            runtime: "claude_code",
            resume_ready: true,
            captured_at: (options.now ?? Date.now)(),
            session_handle: result.sessionId,
            resume_token: result.sessionId,
            checkpoint_id: params.message_id,
            query_ref: result.sessionId,
            fork_available: true,
          },
        };
      }),
    rollback:
      overrides?.rollback ??
      (async (runtime, params) => {
        const checkpointId =
          params.checkpoint_id ??
          (runtime.continuity?.runtime === "claude_code"
            ? runtime.continuity.checkpoint_id
            : undefined);
        if (!checkpointId || typeof runtime.claudeQuery?.rewindFiles !== "function") {
          throw new UnsupportedOperationError("rollback", "claude_code");
        }

        const result = await runtime.claudeQuery.rewindFiles(checkpointId);
        if (result?.canRewind === false) {
          throw new Error(result.error ?? `Unable to rewind Claude files to ${checkpointId}`);
        }
      }),
    revert: overrides?.revert ?? unsupportedOperation("claude_code", "revert"),
    getMessages: overrides?.getMessages ?? unsupportedOperation("claude_code", "getMessages"),
    getDiff: overrides?.getDiff ?? unsupportedOperation("claude_code", "getDiff"),
    executeCommand:
      overrides?.executeCommand ?? unsupportedOperation("claude_code", "executeCommand"),
    interrupt:
      overrides?.interrupt ??
      (async (runtime) => {
        if (typeof runtime.claudeQuery?.interrupt !== "function") {
          throw new UnsupportedOperationError("interrupt", "claude_code");
        }
        await runtime.claudeQuery.interrupt();
      }),
    setModel:
      overrides?.setModel ??
      (async (runtime, params) => {
        if (typeof runtime.claudeQuery?.setModel !== "function") {
          throw new UnsupportedOperationError("setModel", "claude_code");
        }
        await runtime.claudeQuery.setModel(params.model);
      }),
  };
}

function createCodexAdvancedOperations(options: {
  defaultCommand: string;
  defaultModel?: string;
  now?: () => number;
  activePlugins?: ClaudeRuntimeDeps["activePlugins"];
  advancedOperations?: RuntimeAdvancedOperations;
  codexForkRunner?: AgentRuntimeRegistryOptions["codexForkRunner"];
}): RequiredRuntimeAdvancedOperations {
  const overrides = options.advancedOperations;
  return {
    fork:
      overrides?.fork ??
      (async (runtime) => {
        const threadId =
          runtime.continuity?.runtime === "codex"
            ? runtime.continuity.thread_id
            : undefined;
        if (!threadId) {
          throw new UnsupportedOperationError("fork", "codex");
        }

        const runner = options.codexForkRunner ?? defaultCodexForkRunner;
        const result = await runner({
          command: options.defaultCommand,
          threadId,
          cwd: runtime.request?.worktree_path,
        });
        return {
          continuity: {
            runtime: "codex",
            resume_ready: true,
            captured_at: (options.now ?? Date.now)(),
            thread_id: result.threadId,
            fork_available: true,
            rollback_turns: 0,
          },
        };
      }),
    rollback: overrides?.rollback ?? unsupportedOperation("codex", "rollback"),
    revert: overrides?.revert ?? unsupportedOperation("codex", "revert"),
    getMessages: overrides?.getMessages ?? unsupportedOperation("codex", "getMessages"),
    getDiff: overrides?.getDiff ?? unsupportedOperation("codex", "getDiff"),
    executeCommand:
      overrides?.executeCommand ?? unsupportedOperation("codex", "executeCommand"),
    interrupt: overrides?.interrupt ?? unsupportedOperation("codex", "interrupt"),
    setModel: overrides?.setModel ?? unsupportedOperation("codex", "setModel"),
  };
}

function createOpenCodeAdvancedOperations(options: {
  transport: OpenCodeTransport;
  eventRunner?: OpenCodeEventRunner;
  defaultModel?: string;
  now?: () => number;
  advancedOperations?: RuntimeAdvancedOperations;
}): RequiredRuntimeAdvancedOperations {
  const overrides = options.advancedOperations;
  return {
    fork:
      overrides?.fork ??
      (async (runtime, params) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("fork", "opencode");
        }
        const result = await options.transport.forkSession(sessionId, params.message_id);
        return {
          continuity: {
            runtime: "opencode",
            resume_ready: true,
            captured_at: (options.now ?? Date.now)(),
            upstream_session_id: result.id,
            server_url: options.transport.serverUrl,
            fork_available: true,
            revert_message_ids: [],
          },
        };
      }),
    rollback: overrides?.rollback ?? unsupportedOperation("opencode", "rollback"),
    revert:
      overrides?.revert ??
      (async (runtime, params) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("revert", "opencode");
        }

        if (params.action === "unrevert") {
          await options.transport.unrevertMessages(sessionId);
          return;
        }

        if (!params.message_id) {
          throw new Error("message_id is required for OpenCode revert");
        }
        await options.transport.revertMessage(sessionId, params.message_id);
      }),
    getMessages:
      overrides?.getMessages ??
      (async (runtime) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("getMessages", "opencode");
        }
        return options.transport.getMessages(sessionId);
      }),
    getDiff:
      overrides?.getDiff ??
      (async (runtime) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("getDiff", "opencode");
        }
        return options.transport.getDiff(sessionId);
      }),
    executeCommand:
      overrides?.executeCommand ??
      (async (runtime, params) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("executeCommand", "opencode");
        }
        return options.transport.executeCommand(sessionId, params.command, params.arguments);
      }),
    interrupt: overrides?.interrupt ?? unsupportedOperation("opencode", "interrupt"),
    setModel:
      overrides?.setModel ??
      (async (runtime, params) => {
        const provider = runtime.request?.provider ?? "opencode";
        await options.transport.updateConfig({ provider, model: params.model });
      }),
  };
}

function unsupportedOperation(
  runtime: AgentRuntimeKey,
  operation: RuntimeOperationName,
) {
  return async () => {
    throw new UnsupportedOperationError(operation, runtime);
  };
}

function getSupportedFeatures(runtime: AgentRuntimeKey): string[] {
  switch (runtime) {
    case "claude_code":
      return [
        "structured_output",
        "agents",
        "hooks",
        "thinking",
        "file_checkpointing",
        "elicitation",
        "tool_permission_callback",
        "partial_messages",
        "disallowed_tools",
        "fallback_model",
        "additional_directories",
        "env",
        "rate_limit",
        "progress",
        "interrupt",
        "set_model",
        "fork",
        "rollback",
      ];
    case "codex":
      return [
        "turn_started",
        "turn_failed",
        "progress",
        "reasoning",
        "file_change",
        "mcp_tool_call",
        "web_search",
        "todo_update",
        "output_schema",
        "image_attachments",
        "additional_directories",
        "env",
        "mcp_config",
        "fork",
      ];
    case "opencode":
      return [
        "fork",
        "revert",
        "diff",
        "todo_update",
        "messages",
        "command",
        "permission_response",
        "agents",
        "skills",
        "session_status",
        "message_updated",
        "command_executed",
        "vcs_branch_updated",
        "reasoning",
        "file_change",
        "agent_part",
        "compaction",
        "subtask",
        "set_model",
      ];
    default:
      return [];
  }
}

function resolveRuntimeKey(
  req: ExecuteRequest,
  defaultRuntime: AgentRuntimeKey,
): AgentRuntimeKey {
  if (req.runtime) {
    validateRuntimeKey(req.runtime);
    return req.runtime;
  }

  if (!req.provider) {
    return defaultRuntime;
  }

  switch (normalizeProvider(req.provider)) {
    case "anthropic":
      return "claude_code";
    case "codex":
      return "codex";
    case "opencode":
      return "opencode";
    default:
      throw new UnsupportedRuntimeProviderError(
        `Provider ${req.provider} does not support agent_execution`,
      );
  }
}

function validateRuntimeProvider(
  runtime: AgentRuntimeKey,
  provider: string,
  compatibleProviders: string[],
): void {
  if (compatibleProviders.includes(provider)) {
    return;
  }
  throw new UnsupportedRuntimeProviderError(
    `Runtime ${runtime} is incompatible with provider ${provider}`,
  );
}

function assertDiagnosticsAvailable(
  runtime: AgentRuntimeKey,
  diagnostics: RuntimeDiagnostic[],
): void {
  const blocking = diagnostics.find((diagnostic) => diagnostic.blocking);
  if (blocking) {
    throw new RuntimeConfigurationError(blocking.message);
  }
  if (!diagnostics.length) {
    return;
  }
  throw new RuntimeConfigurationError(`Runtime ${runtime} is not available`);
}

function normalizeProvider(provider: string | undefined): string {
  return provider?.trim().toLowerCase() ?? "";
}

function validateRuntimeKey(runtime: string): asserts runtime is AgentRuntimeKey {
  if (runtime !== "claude_code" && runtime !== "codex" && runtime !== "opencode") {
    throw new UnknownRuntimeError(`Unknown runtime: ${runtime}`);
  }
}

function defaultExecutableLookup(command: string): string | null {
  const trimmed = command.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.includes("\\") || trimmed.includes("/") || trimmed.endsWith(".exe")) {
    return existsSync(trimmed) ? trimmed : null;
  }

  const cmd = process.platform === "win32" ? ["where", trimmed] : ["which", trimmed];
  const result = Bun.spawnSync({
    cmd,
    stdout: "pipe",
    stderr: "ignore",
  });
  if (result.exitCode !== 0) {
    return null;
  }

  const output = Buffer.from(result.stdout).toString("utf8").trim();
  const firstLine = output.split(/\r?\n/).find((line) => line.trim().length > 0);
  return firstLine?.trim() || null;
}

async function defaultForkSessionRunner(
  sessionId: string,
  options?: {
    dir?: string;
    upToMessageId?: string;
    title?: string;
  },
): Promise<{ sessionId: string }> {
  const sdk = await import("@anthropic-ai/claude-agent-sdk");
  return sdk.forkSession(sessionId, options);
}

export async function defaultCodexForkRunner(
  params: {
    command: string;
    threadId: string;
    cwd?: string;
  },
  deps: DefaultCodexForkRunnerDeps = {},
): Promise<{ threadId: string }> {
  const sessionsRoot =
    deps.getSessionsRoot?.() ??
    join(process.env.CODEX_HOME ?? join(homedir(), ".codex"), "sessions");
  const listRolloutFiles = deps.listRolloutFiles ?? defaultListCodexRolloutFiles;
  const readRolloutMeta = deps.readRolloutMeta ?? defaultReadCodexRolloutMeta;
  const sleep = deps.sleep ?? defaultSleep;
  const spawn = deps.spawn ?? ((spawnParams) => Bun.spawn(spawnParams));
  const timeoutMs = deps.timeoutMs ?? 5_000;

  const knownFiles = new Set(listRolloutFiles(sessionsRoot));
  const cmd = [params.command, "fork", params.threadId];
  if (params.cwd) {
    cmd.push("-C", params.cwd);
  }
  cmd.push("--no-alt-screen");

  const proc = spawn({
    cmd,
    cwd: params.cwd,
    stdin: "ignore",
    stdout: "pipe",
    stderr: "pipe",
  });
  const stdoutPromise = proc.stdout ? readStreamToString(proc.stdout) : Promise.resolve("");
  const stderrPromise = proc.stderr ? readStreamToString(proc.stderr) : Promise.resolve("");
  const exitPromise = proc.exited;
  const startedAt = Date.now();
  let terminated = false;

  const terminate = async () => {
    if (terminated) {
      return;
    }
    terminated = true;
    try {
      proc.kill();
    } catch {
      // Best effort only.
    }
    await exitPromise.catch(() => undefined);
  };

  try {
    while (Date.now() - startedAt < timeoutMs) {
      const forkedThreadId = findForkedThreadId({
        sessionsRoot,
        sourceThreadId: params.threadId,
        knownFiles,
        listRolloutFiles,
        readRolloutMeta,
      });
      if (forkedThreadId) {
        await terminate();
        return { threadId: forkedThreadId };
      }

      const exitResult = await Promise.race([
        exitPromise.then((exitCode) => ({ done: true as const, exitCode })),
        sleep(100).then(() => ({ done: false as const })),
      ]);
      if (exitResult.done) {
        break;
      }
    }
  } finally {
    await terminate();
  }

  const forkedThreadId = findForkedThreadId({
    sessionsRoot,
    sourceThreadId: params.threadId,
    knownFiles,
    listRolloutFiles,
    readRolloutMeta,
  });
  if (forkedThreadId) {
    return { threadId: forkedThreadId };
  }

  const combinedOutput = `${await stdoutPromise}\n${await stderrPromise}`.trim();
  const outputThreadId = parseThreadIdFromForkOutput(combinedOutput, params.threadId);
  if (outputThreadId) {
    return { threadId: outputThreadId };
  }

  throw new Error(
    combinedOutput
      ? `Unable to determine forked Codex thread id. Output: ${combinedOutput}`
      : "Unable to determine forked Codex thread id from rollout files or command output",
  );
}

function defaultListCodexRolloutFiles(root: string): string[] {
  if (!existsSync(root)) {
    return [];
  }

  const files: string[] = [];
  const stack = [root];
  while (stack.length > 0) {
    const dir = stack.pop();
    if (!dir) {
      continue;
    }

    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const entryPath = join(dir, entry.name);
      if (entry.isDirectory()) {
        stack.push(entryPath);
        continue;
      }

      if (entry.isFile() && /^rollout-.*\.jsonl$/i.test(entry.name)) {
        files.push(entryPath);
      }
    }
  }

  return files;
}

function defaultReadCodexRolloutMeta(filePath: string): CodexRolloutMeta | null {
  try {
    const firstLine = readFileSync(filePath, "utf8")
      .split(/\r?\n/)
      .find((line) => line.trim().length > 0);
    if (!firstLine) {
      return null;
    }

    const parsed = JSON.parse(firstLine);
    if (!isRecord(parsed)) {
      return null;
    }

    const payload = isRecord(parsed.payload) ? parsed.payload : null;
    return {
      threadId:
        typeof payload?.id === "string"
          ? payload.id
          : extractThreadIdFromRolloutPath(filePath),
      forkedFromId:
        typeof payload?.forked_from_id === "string" ? payload.forked_from_id : undefined,
    };
  } catch {
    return null;
  }
}

function findForkedThreadId(params: {
  sessionsRoot: string;
  sourceThreadId: string;
  knownFiles: Set<string>;
  listRolloutFiles: (root: string) => string[];
  readRolloutMeta: (filePath: string) => CodexRolloutMeta | null;
}): string | undefined {
  const addedFiles = params
    .listRolloutFiles(params.sessionsRoot)
    .filter((filePath) => !params.knownFiles.has(filePath));

  const addedMetas = addedFiles
    .map((filePath) => ({
      filePath,
      meta: params.readRolloutMeta(filePath),
    }))
    .flatMap(({ filePath, meta }) => {
      const threadId = meta?.threadId ?? extractThreadIdFromRolloutPath(filePath);
      if (!threadId || threadId === params.sourceThreadId) {
        return [];
      }

      return [
        {
          filePath,
          threadId,
          forkedFromId: meta?.forkedFromId,
        },
      ];
    });

  const directMatch = addedMetas.find(
    (entry) => entry.forkedFromId === params.sourceThreadId,
  );
  if (directMatch) {
    return directMatch.threadId;
  }

  return addedMetas.at(-1)?.threadId;
}

function extractThreadIdFromRolloutPath(filePath: string): string | undefined {
  const match = basename(filePath).match(
    /^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f-]{36})\.jsonl$/i,
  );
  return match?.[1];
}

function parseThreadIdFromForkOutput(
  output: string,
  sourceThreadId: string,
): string | undefined {
  const resumeMatch = output.match(
    /\bresume\s+([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\b/i,
  );
  if (resumeMatch && resumeMatch[1] !== sourceThreadId) {
    return resumeMatch[1];
  }

  const matches =
    output.match(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi) ?? [];
  return matches.find((match) => match !== sourceThreadId);
}

async function defaultSleep(ms: number): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
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

async function readStreamToString(stream: ReadableStream<Uint8Array>): Promise<string> {
  let output = "";
  for await (const line of readLines(stream)) {
    output += `${line}\n`;
  }
  return output.trim();
}

function readEnvConfig(
  envLookup: (name: string) => string | undefined,
  name: string,
): string | undefined {
  const value = envLookup(name)?.trim();
  return value ? value : undefined;
}
