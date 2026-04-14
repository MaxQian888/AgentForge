import { existsSync, readFileSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import { basename, join } from "node:path";
import {
  streamClaudeRuntime,
  type ClaudeRuntimeDeps,
} from "../handlers/claude-runtime.js";
import {
  streamCommandRuntime,
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
  type OpenCodeRuntimeDeps,
} from "../handlers/opencode-runtime.js";
import {
  createOpenCodeTransport,
  type OpenCodeExecuteCapabilities,
  type OpenCodeTransport,
} from "../opencode/transport.js";
import {
  getRuntimeProfile,
  getRuntimeProfiles,
  type CliRuntimeLaunchContract as ProfileCliRuntimeLaunchContract,
  type RuntimeProfile,
  type RuntimeLifecycleMetadata as ProfileRuntimeLifecycleMetadata,
} from "./backend-profiles.js";
import type { AgentRuntime } from "./agent-runtime.js";
import type {
  ExecuteRequest,
  AgentRuntimeKey,
  RuntimeCatalog,
  RuntimeCatalogEntry,
  RuntimeCatalogProvider,
  RuntimeCapabilityDescriptor,
  RuntimeDiagnostic,
  RuntimeContinuityState,
  RuntimeInteractionCapabilities,
  RuntimeLaunchContract,
  RuntimeLifecycleMetadata,
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

export interface RuntimeShellParams {
  command: string;
  agent?: string;
  model?: string;
}

export interface RuntimeThinkingBudgetParams {
  max_thinking_tokens?: number | null;
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
  executeShell?: (runtime: AgentRuntime, params: RuntimeShellParams) => Promise<unknown>;
  setThinkingBudget?: (runtime: AgentRuntime, params: RuntimeThinkingBudgetParams) => Promise<void>;
  getMcpServerStatus?: (runtime: AgentRuntime) => Promise<unknown>;
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
export class ExecuteRequestValidationError extends Error {}

interface RuntimeAdapter {
  key: AgentRuntimeKey;
  label: string;
  defaultProvider: string;
  compatibleProviders: string[];
  defaultModel?: string;
  modelOptions?: string[];
  strictModelOptions: boolean;
  supportedFeatures: string[];
  launchContract?: RuntimeLaunchContract;
  lifecycle?: RuntimeLifecycleMetadata;
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
  executeShell(runtime: AgentRuntime, params: RuntimeShellParams): Promise<unknown>;
  setThinkingBudget(runtime: AgentRuntime, params: RuntimeThinkingBudgetParams): Promise<void>;
  getMcpServerStatus(runtime: AgentRuntime): Promise<unknown>;
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
  opencodePendingInteractions?: OpenCodeRuntimeDeps["opencodePendingInteractions"];
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
  codexRollbackRunner?: (params: {
    command: string;
    threadId: string;
    cwd?: string;
    turns?: number;
    checkpointId?: string;
  }) => Promise<{ threadId?: string; rollbackTurns?: number }>;
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
    validateExecuteInteractionRequest(runtimeKey, req, adapter);

    const provider = normalizeProvider(req.provider) || adapter.defaultProvider;
    validateRuntimeProvider(runtimeKey, provider, adapter.compatibleProviders);
    validateRuntimeModel(runtimeKey, req.model, adapter.modelOptions, adapter.strictModelOptions);
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
        const catalogDetails = adapter.getCatalogDetails
          ? (await adapter.getCatalogDetails()) as Partial<RuntimeCatalogEntry> & {
              extraDiagnostics?: RuntimeDiagnostic[];
            }
          : {};
        const mergedDiagnostics = [
          ...diagnostics,
          ...(catalogDetails.extraDiagnostics ?? []),
        ];
        const runtimeDetails = Object.fromEntries(
          Object.entries(catalogDetails).filter(([key]) => key !== "extraDiagnostics"),
        ) as Partial<RuntimeCatalogEntry>;
        return {
          key: adapter.key,
          label: adapter.label,
          defaultProvider: adapter.defaultProvider,
          compatibleProviders: [...adapter.compatibleProviders],
          defaultModel: adapter.defaultModel,
          modelOptions: adapter.modelOptions,
          available: !mergedDiagnostics.some((diagnostic) => diagnostic.blocking),
          diagnostics: mergedDiagnostics,
          supportedFeatures: [...adapter.supportedFeatures],
          interactionCapabilities: buildInteractionCapabilities(
            adapter,
            mergedDiagnostics,
            runtimeDetails.providers,
          ),
          ...runtimeDetails,
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

  async executeShell(
    runtime: AgentRuntime,
    params: RuntimeShellParams,
  ): Promise<unknown> {
    return this.getAdapterForRuntime(runtime).executeShell(runtime, params);
  }

  async setThinkingBudget(
    runtime: AgentRuntime,
    params: RuntimeThinkingBudgetParams,
  ): Promise<void> {
    await this.getAdapterForRuntime(runtime).setThinkingBudget(runtime, params);
  }

  async getMcpServerStatus(runtime: AgentRuntime): Promise<unknown> {
    return this.getAdapterForRuntime(runtime).getMcpServerStatus(runtime);
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
  const codexProfile = getRuntimeProfile("codex");
  const codexCommand =
    readProfileCommand(envLookup, codexProfile.command) ??
    codexProfile.command?.default_command ??
    "codex";
  const opencodeTransport =
    options.opencodeTransport ??
    createOpenCodeTransport({
      envLookup,
    });
  const codexAuthStatusProvider =
    options.codexAuthStatusProvider ?? (() => getDefaultCodexAuthStatus(codexCommand));

  const claudeProfile = getRuntimeProfile("claude_code");
  const opencodeProfile = getRuntimeProfile("opencode");

  const adapters: Record<AgentRuntimeKey, RuntimeAdapter> = {
    claude_code: {
      key: claudeProfile.key,
      label: claudeProfile.label,
      defaultProvider: claudeProfile.default_provider,
      compatibleProviders: [...claudeProfile.compatible_providers],
      defaultModel:
        readEnvConfig(envLookup, "CLAUDE_CODE_RUNTIME_MODEL") ?? claudeProfile.default_model,
      modelOptions: claudeProfile.model_options,
      strictModelOptions: Boolean(claudeProfile.strict_model_options),
      supportedFeatures: [...claudeProfile.supported_features],
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
        assertDiagnosticsAvailable(await this.getDiagnostics());
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
    codex: createCodexAdapter(codexProfile, {
      executableLookup,
      authStatusProvider: codexAuthStatusProvider,
      codexRuntimeRunner: options.codexRuntimeRunner,
      defaultCommand: codexCommand,
      defaultModel: readEnvConfig(envLookup, "CODEX_RUNTIME_MODEL") ?? codexProfile.default_model,
      now: options.now,
      activePlugins: options.activePlugins,
      advancedOperations: options.advancedOperations?.codex,
      codexForkRunner: options.codexForkRunner,
      codexRollbackRunner: options.codexRollbackRunner,
    }),
    opencode: createOpenCodeReadinessAdapter(opencodeProfile, {
      transport: opencodeTransport,
      eventRunner: options.opencodeEventRunner,
      defaultModel:
        readEnvConfig(envLookup, "OPENCODE_RUNTIME_MODEL") ?? opencodeProfile.default_model,
      now: options.now,
      advancedOperations: options.advancedOperations?.opencode,
      opencodePendingInteractions: options.opencodePendingInteractions,
    }),
    cursor: createCliRuntimeAdapter(getRuntimeProfile("cursor"), {
      executableLookup,
      envLookup,
      commandRuntimeRunner: options.commandRuntimeRunner,
      now: options.now,
    }),
    gemini: createCliRuntimeAdapter(getRuntimeProfile("gemini"), {
      executableLookup,
      envLookup,
      commandRuntimeRunner: options.commandRuntimeRunner,
      now: options.now,
    }),
    qoder: createCliRuntimeAdapter(getRuntimeProfile("qoder"), {
      executableLookup,
      envLookup,
      commandRuntimeRunner: options.commandRuntimeRunner,
      now: options.now,
    }),
    iflow: createCliRuntimeAdapter(getRuntimeProfile("iflow"), {
      executableLookup,
      envLookup,
      commandRuntimeRunner: options.commandRuntimeRunner,
      now: options.now,
    }),
  };

  return new AgentRuntimeRegistry(adapters, options.defaultRuntime ?? "claude_code");
}

function createCodexAdapter(profile: RuntimeProfile, options: {
  executableLookup: (command: string) => string | null;
  authStatusProvider: CodexAuthStatusProvider;
  codexRuntimeRunner?: CodexRuntimeRunner;
  defaultCommand: string;
  defaultModel?: string;
  now?: () => number;
  activePlugins?: ClaudeRuntimeDeps["activePlugins"];
  advancedOperations?: RuntimeAdvancedOperations;
  codexForkRunner?: AgentRuntimeRegistryOptions["codexForkRunner"];
  codexRollbackRunner?: AgentRuntimeRegistryOptions["codexRollbackRunner"];
}): RuntimeAdapter {
  const supportedFeatures = withSupportedFeatures(
    profile.supported_features,
    options.advancedOperations?.rollback || options.codexRollbackRunner ? ["rollback"] : [],
  );
  return {
    key: profile.key,
    label: profile.label,
    defaultProvider: profile.default_provider,
    compatibleProviders: [...profile.compatible_providers],
    defaultModel: options.defaultModel,
    modelOptions: profile.model_options,
    strictModelOptions: Boolean(profile.strict_model_options),
    supportedFeatures,
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
      assertDiagnosticsAvailable(await this.getDiagnostics());
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

function createOpenCodeReadinessAdapter(profile: RuntimeProfile, options: {
  transport: OpenCodeTransport;
  eventRunner?: OpenCodeEventRunner;
  defaultModel?: string;
  now?: () => number;
  advancedOperations?: RuntimeAdvancedOperations;
  opencodePendingInteractions?: OpenCodeRuntimeDeps["opencodePendingInteractions"];
}): RuntimeAdapter {
  const executeCapabilities = getOpenCodeExecuteCapabilities(options.transport);
  const supportedFeatures = withSupportedFeatures(profile.supported_features, [
    ...(executeCapabilities.attachments ? ["attachments"] : []),
    ...(executeCapabilities.env ? ["env"] : []),
    ...(executeCapabilities.web_search ? ["web_search"] : []),
    ...(executeCapabilities.rollback ? ["rollback"] : []),
  ]);
  return {
    key: profile.key,
    label: profile.label,
    defaultProvider: profile.default_provider,
    compatibleProviders: [...profile.compatible_providers],
    defaultModel: options.defaultModel,
    modelOptions: profile.model_options,
    strictModelOptions: Boolean(profile.strict_model_options),
    supportedFeatures,
    async getDiagnostics() {
      const readiness = await options.transport.checkReadiness({
        provider: profile.default_provider,
        model: options.defaultModel,
      });
      return readiness.diagnostics;
    },
    async getCatalogDetails() {
      const details: Partial<RuntimeCatalogEntry> & {
        extraDiagnostics?: RuntimeDiagnostic[];
      } = {};
      const extraDiagnostics: RuntimeDiagnostic[] = [];

      if (typeof options.transport.getAgents === "function") {
        try {
          details.agents = await options.transport.getAgents();
        } catch (error) {
          extraDiagnostics.push({
            code: "catalog_agents_unavailable",
            message: `OpenCode agent discovery failed: ${error instanceof Error ? error.message : String(error)}`,
            blocking: false,
          });
        }
      }

      if (typeof options.transport.getSkills === "function") {
        try {
          details.skills = await options.transport.getSkills();
        } catch (error) {
          extraDiagnostics.push({
            code: "catalog_skills_unavailable",
            message: `OpenCode skill discovery failed: ${error instanceof Error ? error.message : String(error)}`,
            blocking: false,
          });
        }
      }

      if (typeof options.transport.getProviderCatalog === "function") {
        try {
          details.providers = mapOpenCodeProviders(
            await options.transport.getProviderCatalog(),
          );
        } catch (error) {
          extraDiagnostics.push({
            code: "catalog_providers_unavailable",
            message: `OpenCode provider catalog discovery failed: ${error instanceof Error ? error.message : String(error)}`,
            blocking: false,
          });
        }
      }

      if (extraDiagnostics.length > 0) {
        details.extraDiagnostics = extraDiagnostics;
      }

      return details;
    },
    async ensureAvailable() {
      assertDiagnosticsAvailable(await this.getDiagnostics());
    },
    async execute(runtime, streamer, req, systemPrompt) {
      await streamOpenCodeRuntime(runtime, streamer, req, systemPrompt, {
        transport: options.transport,
        eventRunner: options.eventRunner,
        now: options.now,
        opencodePendingInteractions: options.opencodePendingInteractions,
      });
    },
    ...createOpenCodeAdvancedOperations(options),
  };
}

function createCliRuntimeAdapter(
  profile: RuntimeProfile,
  options: {
    executableLookup: (command: string) => string | null;
    envLookup: (name: string) => string | undefined;
    commandRuntimeRunner?: CommandRuntimeRunner;
    now?: () => number;
  },
): RuntimeAdapter {
  const command =
    readProfileCommand(options.envLookup, profile.command) ??
    profile.command?.default_command ??
    profile.key;
  const launchContract = normalizeCliLaunchContract(profile.cli_launch);
  const lifecycle = resolveRuntimeLifecycle(profile.lifecycle, options.now);

  return {
    key: profile.key,
    label: profile.label,
    defaultProvider: profile.default_provider,
    compatibleProviders: [...profile.compatible_providers],
    defaultModel: profile.default_model,
    modelOptions: profile.model_options,
    strictModelOptions: Boolean(profile.strict_model_options),
    supportedFeatures: [...profile.supported_features],
    launchContract,
    lifecycle,
    async getDiagnostics() {
      const diagnostics: RuntimeDiagnostic[] = [];
      const resolved = options.executableLookup(command);
      if (!resolved) {
        diagnostics.push({
          code: "missing_executable",
          message:
            profile.command?.install_hint && profile.command.install_hint.length > 0
              ? profile.command.install_hint
              : `Executable not found for runtime ${profile.key}`,
          blocking: true,
        });
        return diagnostics;
      }

      if (profile.auth?.mode === "env_any" && !hasAnyEnvValue(options.envLookup, profile.auth.env_vars)) {
        diagnostics.push({
          code: "missing_credentials",
          message:
            profile.auth.message && profile.auth.message.length > 0
              ? profile.auth.message
              : `Authentication is unavailable for runtime ${profile.key}`,
          blocking: true,
        });
      }

      const lifecycleDiagnostic = buildRuntimeLifecycleDiagnostic(
        profile.key,
        lifecycle,
      );
      if (lifecycleDiagnostic) {
        diagnostics.push(lifecycleDiagnostic);
      }

      return diagnostics;
    },
    async getCatalogDetails() {
      return {
        launchContract,
        lifecycle,
      };
    },
    async ensureAvailable() {
      assertDiagnosticsAvailable(await this.getDiagnostics());
    },
    async execute(runtime, streamer, req, systemPrompt) {
      const launch = buildCliRuntimeLaunch(
        profile,
        command,
        req,
        systemPrompt,
      );
      await streamCommandRuntime(runtime, streamer, req, systemPrompt, {
        command,
        commandArgs: launch.commandArgs,
        commandEnv: launch.commandEnv,
        stdinPayload: launch.stdinPayload,
        commandRuntimeRunner: options.commandRuntimeRunner,
        now: options.now,
      });
    },
    fork: unsupportedOperation(profile.key, "fork"),
    rollback: unsupportedOperation(profile.key, "rollback"),
    revert: unsupportedOperation(profile.key, "revert"),
    getMessages: unsupportedOperation(profile.key, "getMessages"),
    getDiff: unsupportedOperation(profile.key, "getDiff"),
    executeCommand: unsupportedOperation(profile.key, "executeCommand"),
    executeShell: unsupportedOperation(profile.key, "executeShell"),
    setThinkingBudget: unsupportedOperation(profile.key, "setThinkingBudget"),
    getMcpServerStatus: unsupportedOperation(profile.key, "getMcpServerStatus"),
    interrupt: unsupportedOperation(profile.key, "interrupt"),
    setModel: unsupportedOperation(profile.key, "setModel"),
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
    executeShell:
      overrides?.executeShell ?? unsupportedOperation("claude_code", "executeShell"),
    setThinkingBudget:
      overrides?.setThinkingBudget ??
      (async (runtime, params) => {
        if (typeof runtime.claudeQuery?.setMaxThinkingTokens !== "function") {
          throw new UnsupportedOperationError("setThinkingBudget", "claude_code");
        }
        await runtime.claudeQuery.setMaxThinkingTokens(
          params.max_thinking_tokens ?? null,
        );
      }),
    getMcpServerStatus:
      overrides?.getMcpServerStatus ??
      (async (runtime) => {
        if (typeof runtime.claudeQuery?.mcpServerStatus !== "function") {
          throw new UnsupportedOperationError("getMcpServerStatus", "claude_code");
        }
        return runtime.claudeQuery.mcpServerStatus();
      }),
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
  codexRollbackRunner?: AgentRuntimeRegistryOptions["codexRollbackRunner"];
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
    rollback:
      overrides?.rollback ??
      (async (runtime, params) => {
        const continuity =
          runtime.continuity?.runtime === "codex" ? runtime.continuity : undefined;
        const threadId = continuity?.thread_id;
        if (!threadId) {
          throw new UnsupportedOperationError(
            "rollback",
            "codex",
            "degraded",
            "missing_continuity_state",
            "Codex rollback requires resumable thread continuity",
          );
        }
        if (!options.codexRollbackRunner) {
          throw new UnsupportedOperationError(
            "rollback",
            "codex",
            "unsupported",
            "native_control_unavailable",
            "Codex rollback is not available in the current connector",
          );
        }
        const result = await options.codexRollbackRunner({
          command: options.defaultCommand,
          threadId,
          cwd: runtime.request?.worktree_path,
          turns: params.turns,
          checkpointId: params.checkpoint_id,
        });
        runtime.continuity = {
          runtime: "codex",
          resume_ready: true,
          captured_at: (options.now ?? Date.now)(),
          thread_id: result.threadId ?? threadId,
          fork_available: continuity.fork_available ?? true,
          rollback_turns:
            typeof result.rollbackTurns === "number"
              ? result.rollbackTurns
              : Math.max(0, (continuity.rollback_turns ?? 0) - (params.turns ?? 0)),
        };
      }),
    revert: overrides?.revert ?? unsupportedOperation("codex", "revert"),
    getMessages: overrides?.getMessages ?? unsupportedOperation("codex", "getMessages"),
    getDiff: overrides?.getDiff ?? unsupportedOperation("codex", "getDiff"),
    executeCommand:
      overrides?.executeCommand ?? unsupportedOperation("codex", "executeCommand"),
    executeShell:
      overrides?.executeShell ?? unsupportedOperation("codex", "executeShell"),
    setThinkingBudget:
      overrides?.setThinkingBudget ??
      unsupportedOperation("codex", "setThinkingBudget"),
    getMcpServerStatus:
      overrides?.getMcpServerStatus ??
      unsupportedOperation("codex", "getMcpServerStatus"),
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
    rollback:
      overrides?.rollback ??
      (async (runtime, params) => {
        const continuity =
          runtime.continuity?.runtime === "opencode" ? runtime.continuity : undefined;
        const sessionId = continuity?.upstream_session_id;
        if (!sessionId) {
          throw new UnsupportedOperationError(
            "rollback",
            "opencode",
            "degraded",
            "missing_continuity_state",
            "OpenCode rollback requires a persisted upstream session binding",
          );
        }
        const targetMessageId = resolveOpenCodeRollbackTarget(continuity, params);
        if (!targetMessageId) {
          throw new UnsupportedOperationError(
            "rollback",
            "opencode",
            "degraded",
            "missing_checkpoint",
            "OpenCode rollback requires a revertable message checkpoint",
          );
        }
        await options.transport.revertMessage(sessionId, targetMessageId);
        const revertIds = continuity.revert_message_ids ?? [];
        const targetIndex = revertIds.indexOf(targetMessageId);
        runtime.continuity = {
          ...continuity,
          captured_at: (options.now ?? Date.now)(),
          latest_message_id: targetMessageId,
          revert_message_ids:
            targetIndex >= 0 ? revertIds.slice(0, targetIndex + 1) : revertIds,
        };
      }),
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
    executeShell:
      overrides?.executeShell ??
      (async (runtime, params) => {
        const sessionId =
          runtime.continuity?.runtime === "opencode"
            ? runtime.continuity.upstream_session_id
            : undefined;
        if (!sessionId) {
          throw new UnsupportedOperationError("executeShell", "opencode");
        }
        return options.transport.executeShell(
          sessionId,
          params.command,
          params.agent ?? runtime.request?.role_config?.role_id ?? runtime.request?.team_role,
          params.model ?? runtime.request?.model,
        );
      }),
    setThinkingBudget:
      overrides?.setThinkingBudget ??
      unsupportedOperation("opencode", "setThinkingBudget"),
    getMcpServerStatus:
      overrides?.getMcpServerStatus ??
      unsupportedOperation("opencode", "getMcpServerStatus"),
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
  reasonCode = "unsupported_operation",
) {
  return async () => {
    throw new UnsupportedOperationError(operation, runtime, "unsupported", reasonCode);
  };
}

function withSupportedFeatures(base: string[], additions: string[]): string[] {
  return Array.from(new Set([...base, ...additions]));
}

function hasSupportedFeature(
  supportedFeatures: string[],
  featureNames: string[],
): boolean {
  return featureNames.some((feature) => supportedFeatures.includes(feature));
}

function getOpenCodeExecuteCapabilities(
  transport: OpenCodeTransport,
): OpenCodeExecuteCapabilities {
  if (typeof transport.getExecuteCapabilities === "function") {
    return transport.getExecuteCapabilities();
  }
  return {
    attachments: false,
    env: false,
    web_search: false,
    rollback: false,
  };
}

function resolveOpenCodeRollbackTarget(
  continuity: Extract<RuntimeContinuityState, { runtime: "opencode" }>,
  params: RuntimeRollbackParams,
): string | undefined {
  if (params.checkpoint_id) {
    return params.checkpoint_id;
  }
  const revertIds = continuity.revert_message_ids ?? [];
  if (typeof params.turns === "number" && params.turns > 0) {
    const index = revertIds.length - params.turns;
    return index >= 0 ? revertIds[index] : undefined;
  }
  return continuity.latest_message_id ?? revertIds.at(-1);
}

function validateExecuteInteractionRequest(
  runtime: AgentRuntimeKey,
  req: ExecuteRequest,
  adapter: RuntimeAdapter,
): void {
  const callbackEnabled =
    Boolean(req.tool_permission_callback) ||
    Boolean(req.hooks_config?.hooks.length);
  const callbackUrl = req.hook_callback_url ?? req.hooks_config?.callback_url;

  if (runtime !== "claude_code" && callbackEnabled) {
    throw new ExecuteRequestValidationError(
      `Runtime ${runtime} does not support Claude callback interactions`,
    );
  }

  if (runtime === "claude_code" && callbackEnabled && !callbackUrl?.trim()) {
    throw new ExecuteRequestValidationError(
      "hook_callback_url is required when Claude callback interactions are enabled",
    );
  }

  const supportedFeatures = adapter.supportedFeatures;
  const paritySensitiveInputs: Array<[boolean, string, string[]]> = [
    [Boolean(req.attachments?.length), "attachments", ["attachments", "image_attachments"]],
    [
      Boolean(req.additional_directories?.length),
      "additional_directories",
      ["additional_directories"],
    ],
    [Boolean(req.env && Object.keys(req.env).length > 0), "env", ["env"]],
    [Boolean(req.web_search), "web_search", ["web_search"]],
  ];

  for (const [enabled, fieldName, featureNames] of paritySensitiveInputs) {
    if (!enabled) {
      continue;
    }
    if (hasSupportedFeature(supportedFeatures, featureNames)) {
      continue;
    }
    throw new ExecuteRequestValidationError(
      `Runtime ${runtime} cannot honor ${fieldName} for execute requests`,
    );
  }

  if (adapter.lifecycle?.stage === "sunset") {
    throw new ExecuteRequestValidationError(
      `Runtime ${runtime} has reached its published sunset date`,
    );
  }

  if (!adapter.launchContract) {
    return;
  }

  const supportedApprovalModes = adapter.launchContract.supportedApprovalModes;
  const requestedPermissionMode = normalizeCliPermissionMode(req.permission_mode);
  if (!supportedApprovalModes.includes(requestedPermissionMode)) {
    throw new ExecuteRequestValidationError(
      `Runtime ${runtime} does not support permission_mode=${req.permission_mode} in its documented headless contract`,
    );
  }
}

function mapOpenCodeProviders(catalog: {
  availableProviders: string[];
  connectedProviders: string[];
  defaultModels: Record<string, string>;
  providerModels: Record<string, string[]>;
  authMethods: Record<string, string[]>;
}): RuntimeCatalogProvider[] {
  return catalog.availableProviders.map((provider) => {
    const authMethods = catalog.authMethods[provider] ?? [];
    const connected = catalog.connectedProviders.includes(provider);
    return {
      provider,
      connected,
      defaultModel: catalog.defaultModels[provider],
      modelOptions: catalog.providerModels[provider] ?? [],
      authRequired: !connected && authMethods.length > 0,
      authMethods,
    } satisfies RuntimeCatalogProvider;
  });
}

function buildInteractionCapabilities(
  adapter: RuntimeAdapter,
  diagnostics: RuntimeDiagnostic[],
  providers: RuntimeCatalogProvider[] | undefined,
): RuntimeInteractionCapabilities {
  const blocking = diagnostics.find((diagnostic) => diagnostic.blocking);
  const readinessReasonCode = blocking?.code;
  const readinessMessage = blocking?.message;

  const supported = (
    options: {
      requiresRequestFields?: string[];
      degradeWithReadiness?: boolean;
      reasonCode?: string;
      message?: string;
    } = {},
  ): RuntimeCapabilityDescriptor => ({
    state:
      options.degradeWithReadiness !== false && blocking ? "degraded" : "supported",
    reasonCode:
      options.degradeWithReadiness !== false && blocking
        ? readinessReasonCode
        : options.reasonCode,
    message:
      options.degradeWithReadiness !== false && blocking
        ? readinessMessage
        : options.message,
    requiresRequestFields: options.requiresRequestFields,
  });

  const unsupported = (
    reasonCode: string,
    message?: string,
  ): RuntimeCapabilityDescriptor => ({
    state: "unsupported",
    reasonCode,
    message,
  });

  const featureSupport = (
    featureNames: string[],
    options?: Parameters<typeof supported>[0],
  ): RuntimeCapabilityDescriptor =>
    hasSupportedFeature(adapter.supportedFeatures, featureNames)
      ? supported(options)
      : unsupported("not_supported");

  const inputs: Record<string, RuntimeCapabilityDescriptor> = {
    structured_output: featureSupport(["structured_output", "output_schema"]),
    attachments: featureSupport(["attachments", "image_attachments"]),
    additional_directories: featureSupport(["additional_directories"]),
    env: featureSupport(["env"]),
    web_search: featureSupport(["web_search"]),
    agents: featureSupport(["agents"]),
    hooks: featureSupport(["hooks"], { requiresRequestFields: ["hook_callback_url"] }),
    thinking: featureSupport(["thinking"]),
  };
  const lifecycle: Record<string, RuntimeCapabilityDescriptor> = {
    execute: supported(),
    fork: featureSupport(["fork"]),
    rollback: featureSupport(["rollback"]),
    revert: featureSupport(["revert"]),
    diff: featureSupport(["diff"]),
    messages: featureSupport(["messages"]),
    command: featureSupport(["command"]),
    shell: featureSupport(["shell"]),
    interrupt: featureSupport(["interrupt"]),
    set_model: featureSupport(["set_model"]),
    set_thinking_budget: unsupported("not_supported"),
    mcp_status: unsupported("not_supported"),
  };
  const approval: Record<string, RuntimeCapabilityDescriptor> = {
    hooks: featureSupport(["hooks"], { requiresRequestFields: ["hook_callback_url"] }),
    tool_permission_callback: featureSupport(
      ["tool_permission_callback"],
      { requiresRequestFields: ["hook_callback_url"] },
    ),
    provider_auth: unsupported("not_supported"),
    permission_response: featureSupport(
      ["permission_response"],
      { requiresRequestFields: ["hook_callback_url"] },
    ),
  };
  const mcp: Record<string, RuntimeCapabilityDescriptor> = {
    config_overlay: featureSupport(["mcp_config"]),
    runtime_servers: unsupported("not_supported"),
  };

  switch (adapter.key) {
    case "claude_code":
      lifecycle.set_thinking_budget = supported();
      lifecycle.mcp_status = supported();
      approval.permission_response = supported({
        requiresRequestFields: ["hook_callback_url"],
      });
      mcp.runtime_servers = supported();
      break;
    case "codex":
      approval.provider_auth = supported();
      mcp.runtime_servers = supported();
      break;
    case "opencode":
      approval.provider_auth =
        diagnostics.some((diagnostic) => diagnostic.code === "catalog_providers_unavailable")
          ? {
              state: "degraded",
              reasonCode: "catalog_providers_unavailable",
              message:
                diagnostics.find((diagnostic) => diagnostic.code === "catalog_providers_unavailable")
                  ?.message ?? "OpenCode provider catalog discovery is unavailable",
            }
          : providers?.some((provider) => provider.authRequired)
          ? {
              state: "degraded",
              reasonCode: "provider_auth_required",
              message:
                "One or more OpenCode providers require authentication. Start via /bridge/opencode/provider-auth/:provider/start",
            }
          : supported();
      break;
    default:
      break;
  }

  return {
    inputs,
    lifecycle,
    approval,
    mcp,
    diagnostics: {
      readiness: {
        state: blocking ? "degraded" : "supported",
        reasonCode: readinessReasonCode,
        message: readinessMessage,
      },
    },
  };
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
    case "cursor":
      return "cursor";
    case "google":
    case "vertex":
      return "gemini";
    case "qoder":
      return "qoder";
    case "iflow":
      return "iflow";
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

function validateRuntimeModel(
  runtime: AgentRuntimeKey,
  model: string | undefined,
  modelOptions: string[] | undefined,
  strictModelOptions: boolean,
): void {
  if (!strictModelOptions || !model || !modelOptions || modelOptions.length === 0) {
    return;
  }

  if (modelOptions.includes(model)) {
    return;
  }

  throw new RuntimeConfigurationError(
    `Runtime ${runtime} does not support model ${model}`,
  );
}

function assertDiagnosticsAvailable(diagnostics: RuntimeDiagnostic[]): void {
  const blocking = diagnostics.find((diagnostic) => diagnostic.blocking);
  if (blocking) {
    throw new RuntimeConfigurationError(blocking.message);
  }
  return;
}

function normalizeProvider(provider: string | undefined): string {
  return provider?.trim().toLowerCase() ?? "";
}

function validateRuntimeKey(runtime: string): asserts runtime is AgentRuntimeKey {
  if (!getRuntimeProfiles().some((profile) => profile.key === runtime)) {
    throw new UnknownRuntimeError(`Unknown runtime: ${runtime}`);
  }
}

function readProfileCommand(
  envLookup: (name: string) => string | undefined,
  command: RuntimeProfile["command"],
): string | undefined {
  if (!command?.env_var) {
    return undefined;
  }
  return readEnvConfig(envLookup, command.env_var);
}

function hasAnyEnvValue(
  envLookup: (name: string) => string | undefined,
  envVars: string[] | undefined,
): boolean {
  return (envVars ?? []).some((name) => Boolean(readEnvConfig(envLookup, name)));
}

function composeCliPrompt(systemPrompt: string, prompt: string): string {
  const trimmedSystemPrompt = systemPrompt.trim();
  if (!trimmedSystemPrompt) {
    return prompt;
  }
  return `${trimmedSystemPrompt}\n\n${prompt}`;
}

function normalizeCliLaunchContract(
  contract: ProfileCliRuntimeLaunchContract | undefined,
): RuntimeLaunchContract | undefined {
  if (!contract) {
    return undefined;
  }
  return {
    promptTransport: contract.prompt_transport,
    outputMode: contract.output_mode,
    supportedOutputModes: [...contract.supported_output_modes],
    supportedApprovalModes: [...contract.supported_approval_modes],
    additionalDirectories: contract.additional_directories,
    envOverrides: contract.env_overrides,
  };
}

function resolveRuntimeLifecycle(
  lifecycle: ProfileRuntimeLifecycleMetadata | undefined,
  now: (() => number) | undefined,
): RuntimeLifecycleMetadata | undefined {
  if (!lifecycle) {
    return undefined;
  }
  const sunsetAt = lifecycle.sunset_at;
  const stage =
    sunsetAt && Number.isFinite(Date.parse(sunsetAt)) && (now ?? Date.now)() >= Date.parse(sunsetAt)
      ? "sunset"
      : lifecycle.stage;
  return {
    stage,
    sunsetAt: sunsetAt,
    replacementRuntime: lifecycle.replacement_runtime,
    message: lifecycle.message,
  };
}

function buildRuntimeLifecycleDiagnostic(
  runtime: AgentRuntimeKey,
  lifecycle: RuntimeLifecycleMetadata | undefined,
): RuntimeDiagnostic | null {
  if (!lifecycle) {
    return null;
  }
  if (lifecycle.stage === "sunset") {
    return {
      code: "runtime_sunset",
      message:
        lifecycle.message ??
        `Runtime ${runtime} has reached its published sunset date.`,
      blocking: true,
    };
  }
  if (lifecycle.stage === "sunsetting") {
    return {
      code: "sunset_window",
      message:
        lifecycle.message ??
        `Runtime ${runtime} is inside its published shutdown window.`,
      blocking: false,
    };
  }
  return null;
}

function normalizeCliPermissionMode(permissionMode: string | undefined): string {
  switch (permissionMode) {
    case "bypassPermissions":
    case "auto":
    case "yolo":
      return "yolo";
    case "acceptEdits":
      return "auto_edit";
    case undefined:
    case "":
      return "default";
    default:
      return permissionMode;
  }
}

function buildCliRuntimeLaunch(
  profile: RuntimeProfile,
  command: string,
  req: ExecuteRequest,
  systemPrompt: string,
): {
  commandArgs: string[];
  commandEnv?: Record<string, string>;
  stdinPayload?: string;
} {
  const prompt = composeCliPrompt(systemPrompt, req.prompt);

  switch (profile.key) {
    case "cursor": {
      const commandArgs = ["-p", "--output-format", "stream-json", "--trust"];
      switch (normalizeCliPermissionMode(req.permission_mode)) {
        case "plan":
          commandArgs.push("--mode", "plan");
          break;
        case "ask":
          commandArgs.push("--mode", "ask");
          break;
        case "yolo":
          commandArgs.push("--force");
          break;
      }
      if (req.model) {
        commandArgs.push("--model", req.model);
      }
      commandArgs.push(prompt);
      return {
        commandArgs,
        stdinPayload: "",
      };
    }
    case "gemini": {
      const commandArgs = ["-p", prompt, "--output-format", "stream-json"];
      switch (normalizeCliPermissionMode(req.permission_mode)) {
        case "plan":
          commandArgs.push("--approval-mode=plan");
          break;
        case "auto_edit":
          commandArgs.push("--approval-mode=auto_edit");
          break;
        case "yolo":
          commandArgs.push("--approval-mode=yolo");
          break;
      }
      if (req.model) {
        commandArgs.push("--model", req.model);
      }
      if (req.additional_directories?.length) {
        commandArgs.push("--include-directories", req.additional_directories.join(","));
      }
      return { commandArgs };
    }
    case "qoder": {
      const commandArgs = ["--print", "-p", prompt, "--output-format", "stream-json"];
      if (normalizeCliPermissionMode(req.permission_mode) === "yolo") {
        commandArgs.push("--yolo");
      }
      if (req.model) {
        commandArgs.push("-m", req.model);
      }
      commandArgs.push("-w", req.worktree_path);
      return { commandArgs };
    }
    case "iflow": {
      const commandArgs = ["--prompt", prompt];
      if (normalizeCliPermissionMode(req.permission_mode) === "yolo") {
        commandArgs.push("--yolo");
      }
      if (req.model) {
        commandArgs.push("--model", req.model);
      }
      for (const directory of req.additional_directories ?? []) {
        commandArgs.push("--add-dir", directory);
      }
      return { commandArgs };
    }
    default:
      return { commandArgs: [command] };
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
