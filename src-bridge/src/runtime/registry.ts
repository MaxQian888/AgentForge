import { existsSync } from "node:fs";
import {
  streamClaudeRuntime,
  type ClaudeRuntimeDeps,
} from "../handlers/claude-runtime.js";
import {
  streamCommandRuntime,
  type CommandRuntimeRunner,
} from "../handlers/command-runtime.js";
import type { AgentRuntime } from "./agent-runtime.js";
import type { ExecuteRequest, AgentRuntimeKey } from "../types.js";
import type { EventStreamer } from "../ws/event-stream.js";

type EventSink = Pick<EventStreamer, "send">;

export class UnknownRuntimeError extends Error {}
export class RuntimeConfigurationError extends Error {}
export class UnsupportedRuntimeProviderError extends Error {}

interface RuntimeAdapter {
  key: AgentRuntimeKey;
  defaultModel?: string;
  ensureAvailable(): void;
  execute(
    runtime: AgentRuntime,
    streamer: EventSink,
    req: ExecuteRequest,
    systemPrompt: string,
  ): Promise<void>;
}

export interface AgentRuntimeRegistryOptions extends ClaudeRuntimeDeps {
  commandRuntimeRunner?: CommandRuntimeRunner;
  defaultRuntime?: AgentRuntimeKey;
  executableLookup?: (command: string) => string | null;
  envLookup?: (name: string) => string | undefined;
}

export class AgentRuntimeRegistry {
  constructor(
    private readonly adapters: Record<AgentRuntimeKey, RuntimeAdapter>,
    private readonly defaultRuntime: AgentRuntimeKey,
  ) {}

  resolveExecute(req: ExecuteRequest): { adapter: RuntimeAdapter; request: ExecuteRequest } {
    const runtimeKey = resolveRuntimeKey(req, this.defaultRuntime);
    const adapter = this.adapters[runtimeKey];
    if (!adapter) {
      throw new UnknownRuntimeError(`Unknown runtime: ${runtimeKey}`);
    }

    adapter.ensureAvailable();

    return {
      adapter,
      request: {
        ...req,
        runtime: runtimeKey,
        model: req.model ?? adapter.defaultModel,
      },
    };
  }
}

export function createRuntimeRegistry(
  options: AgentRuntimeRegistryOptions = {},
): AgentRuntimeRegistry {
  const executableLookup = options.executableLookup ?? defaultExecutableLookup;
  const envLookup = options.envLookup ?? ((name: string) => process.env[name]);

  const adapters: Record<AgentRuntimeKey, RuntimeAdapter> = {
    claude_code: {
      key: "claude_code",
      defaultModel: readEnvConfig(envLookup, "CLAUDE_CODE_RUNTIME_MODEL"),
      ensureAvailable() {
        if (options.queryRunner) {
          return;
        }

        const apiKey = envLookup("ANTHROPIC_API_KEY")?.trim();
        if (!apiKey) {
          throw new RuntimeConfigurationError(
            "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
          );
        }
      },
      async execute(runtime, streamer, req, systemPrompt) {
        await streamClaudeRuntime(runtime, streamer, req, systemPrompt, {
          queryRunner: options.queryRunner,
          now: options.now,
        });
      },
    },
    codex: createCommandAdapter("codex", {
      commandRuntimeRunner: options.commandRuntimeRunner,
      executableLookup,
      defaultCommand: readEnvConfig(envLookup, "CODEX_RUNTIME_COMMAND") || "codex",
      defaultModel: readEnvConfig(envLookup, "CODEX_RUNTIME_MODEL"),
      now: options.now,
    }),
    opencode: createCommandAdapter("opencode", {
      commandRuntimeRunner: options.commandRuntimeRunner,
      executableLookup,
      defaultCommand: readEnvConfig(envLookup, "OPENCODE_RUNTIME_COMMAND") || "opencode",
      defaultModel: readEnvConfig(envLookup, "OPENCODE_RUNTIME_MODEL"),
      now: options.now,
    }),
  };

  return new AgentRuntimeRegistry(adapters, options.defaultRuntime ?? "claude_code");
}

function createCommandAdapter(
  key: "codex" | "opencode",
  options: {
    commandRuntimeRunner?: CommandRuntimeRunner;
    executableLookup: (command: string) => string | null;
    defaultCommand: string;
    defaultModel?: string;
    now?: () => number;
  },
): RuntimeAdapter {
  return {
    key,
    defaultModel: options.defaultModel,
    ensureAvailable() {
      const resolved = options.executableLookup(options.defaultCommand);
      if (!resolved) {
        throw new RuntimeConfigurationError(`Executable not found for runtime ${key}`);
      }
    },
    async execute(runtime, streamer, req, systemPrompt) {
      await streamCommandRuntime(runtime, streamer, req, systemPrompt, {
        command: options.defaultCommand,
        commandRuntimeRunner: options.commandRuntimeRunner,
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

  switch (req.provider) {
    case "anthropic":
    case "claude":
    case "claude_code":
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
