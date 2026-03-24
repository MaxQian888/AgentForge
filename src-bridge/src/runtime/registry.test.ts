import { describe, expect, test } from "bun:test";
import { createRuntimeRegistry } from "./registry.js";
import type { ExecuteRequest } from "../types.js";

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-123",
    session_id: "session-123",
    prompt: "Implement the requested bridge change.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-123",
    system_prompt: "",
    max_turns: 12,
    budget_usd: 5,
    allowed_tools: ["Read"],
    permission_mode: "default",
    ...overrides,
  };
}

describe("agent runtime registry", () => {
  test("publishes runtime catalog metadata and readiness diagnostics", () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return command === "codex" ? "C:/mock/codex.exe" : null;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "";
          case "CLAUDE_CODE_RUNTIME_MODEL":
            return "claude-sonnet-4-5";
          case "CODEX_RUNTIME_MODEL":
            return "gpt-5-codex";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
    });

    const catalog = registry.getCatalog();
    expect(catalog.defaultRuntime).toBe("claude_code");
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "claude_code",
          defaultProvider: "anthropic",
          compatibleProviders: ["anthropic"],
          defaultModel: "claude-sonnet-4-5",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "missing_credentials" }),
          ]),
        }),
        expect.objectContaining({
          key: "codex",
          defaultProvider: "openai",
          compatibleProviders: ["openai", "codex"],
          defaultModel: "gpt-5-codex",
          available: true,
          diagnostics: [],
        }),
        expect.objectContaining({
          key: "opencode",
          defaultProvider: "opencode",
          compatibleProviders: ["opencode"],
          defaultModel: "opencode-default",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "missing_executable" }),
          ]),
        }),
      ]),
    );
  });

  test("uses injected env lookup for runtime defaults and command discovery", () => {
    const lookedUpCommands: string[] = [];
    const previousCodexCommand = process.env.CODEX_RUNTIME_COMMAND;
    const previousCodexModel = process.env.CODEX_RUNTIME_MODEL;

    delete process.env.CODEX_RUNTIME_COMMAND;
    delete process.env.CODEX_RUNTIME_MODEL;

    try {
      const registry = createRuntimeRegistry({
        executableLookup(command) {
          lookedUpCommands.push(command);
          return command === "custom-codex" ? "C:/mock/custom-codex.exe" : null;
        },
        envLookup(name) {
          switch (name) {
            case "ANTHROPIC_API_KEY":
              return "test-token";
            case "CODEX_RUNTIME_COMMAND":
              return "custom-codex";
            case "CODEX_RUNTIME_MODEL":
              return "gpt-5-codex-custom";
            default:
              return undefined;
          }
        },
      });

      const resolved = registry.resolveExecute(createRequest({ runtime: "codex" }));

      expect(resolved.request.model).toBe("gpt-5-codex-custom");
      expect(lookedUpCommands).toEqual(["custom-codex"]);
    } finally {
      if (previousCodexCommand === undefined) {
        delete process.env.CODEX_RUNTIME_COMMAND;
      } else {
        process.env.CODEX_RUNTIME_COMMAND = previousCodexCommand;
      }

      if (previousCodexModel === undefined) {
        delete process.env.CODEX_RUNTIME_MODEL;
      } else {
        process.env.CODEX_RUNTIME_MODEL = previousCodexModel;
      }
    }
  });

  test("defaults omitted runtime to claude_code and maps legacy provider hints", () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup() {
        return "test-token";
      },
    });

    expect(registry.resolveExecute(createRequest()).request.runtime).toBe("claude_code");
    expect(
      registry.resolveExecute(createRequest({ provider: "anthropic" })).request.runtime,
    ).toBe("claude_code");
    expect(registry.resolveExecute(createRequest({ provider: "codex" })).request.runtime).toBe(
      "codex",
    );
    expect(
      registry.resolveExecute(createRequest({ provider: "opencode" })).request.runtime,
    ).toBe("opencode");
    expect(
      registry.resolveExecute(createRequest({ runtime: "codex", provider: "openai" })).request
        .runtime,
    ).toBe("codex");
  });

  test("rejects explicit runtime/provider combinations that are incompatible", () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup() {
        return "test-token";
      },
    });

    expect(() =>
      registry.resolveExecute(
        createRequest({ runtime: "codex", provider: "anthropic", model: "gpt-5-codex" }),
      ),
    ).toThrow("Runtime codex is incompatible with provider anthropic");
  });

  test("rejects unknown runtime hints and missing runtime executables", () => {
    const registry = createRuntimeRegistry({
      executableLookup() {
        return null;
      },
      envLookup() {
        return "test-token";
      },
    });

    expect(() =>
      registry.resolveExecute(createRequest({ runtime: "made_up_runtime" as never })),
    ).toThrow("Unknown runtime: made_up_runtime");
    expect(() => registry.resolveExecute(createRequest({ runtime: "codex" }))).toThrow(
      "Executable not found for runtime codex",
    );
  });

  test("rejects claude_code when the required credential is missing", () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup() {
        return undefined;
      },
    });

    expect(() => registry.resolveExecute(createRequest({ runtime: "claude_code" }))).toThrow(
      "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
    );
  });
});
