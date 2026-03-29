import { existsSync } from "node:fs";
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

type EventSink = Pick<EventStreamer, "send">;

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
  ensureAvailable(): Promise<void>;
  execute(
    runtime: AgentRuntime,
    streamer: EventSink,
    req: ExecuteRequest,
    systemPrompt: string,
  ): Promise<void>;
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
        } satisfies RuntimeCatalogEntry;
      }),
    );

    return {
      defaultRuntime: this.defaultRuntime,
      runtimes,
    };
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
          now: options.now,
        });
      },
    },
    codex: createCodexAdapter({
      executableLookup,
      authStatusProvider: codexAuthStatusProvider,
      codexRuntimeRunner: options.codexRuntimeRunner,
      defaultCommand: codexCommand,
      defaultModel: readEnvConfig(envLookup, "CODEX_RUNTIME_MODEL"),
      now: options.now,
    }),
    opencode: createOpenCodeReadinessAdapter({
      transport: opencodeTransport,
      eventRunner: options.opencodeEventRunner,
      defaultModel: readEnvConfig(envLookup, "OPENCODE_RUNTIME_MODEL"),
      now: options.now,
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
      });
    },
  };
}

function createOpenCodeReadinessAdapter(options: {
  transport: OpenCodeTransport;
  eventRunner?: OpenCodeEventRunner;
  defaultModel?: string;
  now?: () => number;
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

function readEnvConfig(
  envLookup: (name: string) => string | undefined,
  name: string,
): string | undefined {
  const value = envLookup(name)?.trim();
  return value ? value : undefined;
}
