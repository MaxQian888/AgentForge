import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "./agent-runtime.js";
import { createRuntimeRegistry, defaultCodexForkRunner } from "./registry.js";
import type { ExecuteRequest } from "../types.js";
import { UnsupportedOperationError } from "./errors.js";

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
  test("keeps catalog loading when the default Codex auth probe cannot be spawned", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return command === "codex" ? "C:/mock/codex.exe" : null;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: false,
          message: "codex login status blocked",
        };
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({
            ok: true,
            diagnostics: [],
          });
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "codex",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({
              code: "missing_credentials",
              message: expect.stringContaining("codex login status blocked"),
            }),
          ]),
        }),
      ]),
    );
  });

  test("publishes runtime catalog metadata and readiness diagnostics", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return command === "codex" ? "C:/mock/codex.exe" : null;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: false,
          message: "Codex CLI is not logged in",
        };
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
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
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({
            ok: false,
            diagnostics: [
              {
                code: "server_unreachable",
                message: "OpenCode server http://127.0.0.1:4096 is unreachable",
                blocking: true,
              },
            ],
          });
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.defaultRuntime).toBe("claude_code");
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "claude_code",
          defaultProvider: "anthropic",
          compatibleProviders: ["anthropic"],
          defaultModel: "claude-sonnet-4-5",
          supportedFeatures: expect.arrayContaining(["structured_output", "interrupt"]),
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "missing_credentials" }),
          ]),
          interactionCapabilities: expect.objectContaining({
            inputs: expect.objectContaining({
              structured_output: expect.objectContaining({
                state: "degraded",
                reasonCode: "missing_credentials",
              }),
            }),
            approval: expect.objectContaining({
              hooks: expect.objectContaining({
                state: "degraded",
                requiresRequestFields: ["hook_callback_url"],
              }),
            }),
            diagnostics: expect.objectContaining({
              readiness: expect.objectContaining({
                state: "degraded",
                reasonCode: "missing_credentials",
              }),
            }),
          }),
        }),
        expect.objectContaining({
          key: "codex",
          defaultProvider: "openai",
          compatibleProviders: ["openai", "codex"],
          defaultModel: "gpt-5-codex",
          supportedFeatures: expect.arrayContaining(["reasoning", "output_schema", "fork"]),
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "missing_credentials" }),
          ]),
          interactionCapabilities: expect.objectContaining({
            lifecycle: expect.objectContaining({
              fork: expect.objectContaining({
                state: "degraded",
                reasonCode: "missing_credentials",
              }),
            }),
            mcp: expect.objectContaining({
              config_overlay: expect.objectContaining({
                state: "degraded",
                reasonCode: "missing_credentials",
              }),
            }),
          }),
        }),
        expect.objectContaining({
          key: "opencode",
          defaultProvider: "opencode",
          compatibleProviders: ["opencode"],
          defaultModel: "opencode-default",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "server_unreachable" }),
          ]),
          interactionCapabilities: expect.objectContaining({
            lifecycle: expect.objectContaining({
              shell: expect.objectContaining({
                state: "degraded",
                reasonCode: "server_unreachable",
              }),
            }),
            approval: expect.objectContaining({
              provider_auth: expect.objectContaining({
                state: "degraded",
                reasonCode: "server_unreachable",
              }),
            }),
          }),
        }),
      ]),
    );
  });

  test("publishes additional CLI-backed runtime profiles with bounded model options", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        switch (command) {
          case "agent":
          case "gemini":
          case "qodercli":
          case "iflow":
            return `C:/mock/${command}.exe`;
          default:
            return null;
        }
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "GEMINI_API_KEY":
            return "gemini-token";
          case "CURSOR_API_KEY":
            return "cursor-token";
          case "IFLOW_API_KEY":
            return "iflow-token";
          default:
            return undefined;
        }
      },
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "cursor",
          label: "Cursor Agent",
          defaultProvider: "cursor",
          compatibleProviders: ["cursor"],
          modelOptions: expect.arrayContaining(["claude-sonnet-4-20250514", "gpt-4o"]),
          supportedFeatures: expect.arrayContaining(["progress", "reasoning"]),
        }),
        expect.objectContaining({
          key: "gemini",
          label: "Gemini CLI",
          defaultProvider: "google",
          compatibleProviders: ["google", "vertex"],
          modelOptions: expect.arrayContaining(["gemini-2.5-pro", "gemini-2.5-flash"]),
          supportedFeatures: expect.arrayContaining(["reasoning", "plan_mode"]),
        }),
        expect.objectContaining({
          key: "qoder",
          label: "Qoder CLI",
          defaultProvider: "qoder",
          compatibleProviders: ["qoder"],
          modelOptions: expect.arrayContaining(["auto", "ultimate"]),
        }),
        expect.objectContaining({
          key: "iflow",
          label: "iFlow CLI",
          defaultProvider: "iflow",
          compatibleProviders: ["iflow"],
          modelOptions: expect.arrayContaining(["Qwen3-Coder", "Kimi-K2.5"]),
          supportedFeatures: expect.arrayContaining(["plan_mode", "auto_edit"]),
        }),
      ]),
    );
  });

  test("uses injected env lookup for runtime defaults and command discovery", async () => {
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
        codexAuthStatusProvider() {
          return {
            authenticated: true,
            message: "Logged in using an API key",
          };
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

      const resolved = await registry.resolveExecute(createRequest({ runtime: "codex" }));

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

  test("defaults omitted runtime to claude_code and maps legacy provider hints", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in using an API key",
        };
      },
      envLookup() {
        return "test-token";
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({
            ok: true,
            diagnostics: [],
          });
        },
      } as never,
    });

    expect((await registry.resolveExecute(createRequest())).request.runtime).toBe("claude_code");
    expect(
      (await registry.resolveExecute(createRequest({ provider: "anthropic" }))).request.runtime,
    ).toBe("claude_code");
    expect((await registry.resolveExecute(createRequest({ provider: "codex" }))).request.runtime).toBe(
      "codex",
    );
    expect(
      (await registry.resolveExecute(createRequest({ provider: "opencode" }))).request.runtime,
    ).toBe("opencode");
    expect(
      (await registry.resolveExecute(createRequest({ runtime: "codex", provider: "openai" }))).request
        .runtime,
    ).toBe("codex");
  });

  test("reports provider/model readiness failures for opencode from the transport layer", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "missing-model";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({
            ok: false,
            diagnostics: [
              {
                code: "model_unavailable",
                message: "OpenCode model missing-model is not available for provider opencode",
                blocking: true,
              },
            ],
          });
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "opencode",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({ code: "model_unavailable" }),
          ]),
        }),
      ]),
    );

    await expect(
      registry.resolveExecute(createRequest({ runtime: "opencode", provider: "opencode" })),
    ).rejects.toThrow("OpenCode model missing-model is not available for provider opencode");
  });

  test("includes OpenCode agents and skills in runtime catalog metadata", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
        getAgents() {
          return Promise.resolve(["planner", "reviewer"]);
        },
        getSkills() {
          return Promise.resolve(["opsx-apply", "opsx-archive"]);
        },
        getProviderCatalog() {
          return Promise.resolve({
            availableProviders: ["opencode", "anthropic"],
            connectedProviders: ["opencode"],
            defaultModels: {
              opencode: "opencode-default",
              anthropic: "claude-sonnet-4-5",
            },
            providerModels: {
              opencode: ["opencode-default"],
              anthropic: ["claude-sonnet-4-5", "claude-opus-4-1"],
            },
            authMethods: {
              anthropic: ["oauth"],
            },
          });
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "opencode",
          agents: ["planner", "reviewer"],
          skills: ["opsx-apply", "opsx-archive"],
          providers: expect.arrayContaining([
            expect.objectContaining({
              provider: "opencode",
              connected: true,
              defaultModel: "opencode-default",
            }),
            expect.objectContaining({
              provider: "anthropic",
              connected: false,
              authRequired: true,
              authMethods: ["oauth"],
            }),
          ]),
        }),
      ]),
    );
  });

  test("launches cursor through documented headless print mode with positional prompt transport", async () => {
    const launches: Array<{
      runtime: string;
      command: string;
      commandArgs?: string[];
      stdinPayload?: string;
    }> = [];
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        switch (command) {
          case "agent":
            return "C:/mock/agent.exe";
          default:
            return null;
        }
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "CURSOR_API_KEY":
            return "cursor-token";
          default:
            return undefined;
        }
      },
      commandRuntimeRunner: async function* capture(params) {
        launches.push({
          runtime: params.runtime,
          command: params.command,
          commandArgs: params.commandArgs,
          stdinPayload: params.stdinPayload,
        });
      },
    });

    const { adapter, request } = await registry.resolveExecute(
      createRequest({
        runtime: "cursor",
        provider: "cursor",
        model: "gpt-4o",
        permission_mode: "plan",
      }),
    );
    const runtime = new AgentRuntime(request.task_id, request.session_id);
    runtime.bindRequest(request);

    await adapter.execute(runtime, { send() {} }, request, "System prompt");

    expect(launches).toEqual([
      expect.objectContaining({
        runtime: "cursor",
        command: "agent",
        commandArgs: [
          "-p",
          "--output-format",
          "stream-json",
          "--trust",
          "--mode",
          "plan",
          "--model",
          "gpt-4o",
          "System prompt\n\nImplement the requested bridge change.",
        ],
        stdinPayload: "",
      }),
    ]);
  });

  test("launches gemini through documented prompt and output flags with include-directories support", async () => {
    const launches: Array<{
      runtime: string;
      command: string;
      commandArgs?: string[];
      stdinPayload?: string;
    }> = [];
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return command === "gemini" ? "C:/mock/gemini.exe" : null;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "GEMINI_API_KEY":
            return "gemini-token";
          default:
            return undefined;
        }
      },
      commandRuntimeRunner: async function* capture(params) {
        launches.push({
          runtime: params.runtime,
          command: params.command,
          commandArgs: params.commandArgs,
          stdinPayload: params.stdinPayload,
        });
      },
    });

    const { adapter, request } = await registry.resolveExecute(
      createRequest({
        runtime: "gemini",
        provider: "google",
        model: "gemini-2.5-pro",
        permission_mode: "auto_edit",
        additional_directories: ["D:/Project/shared", "D:/Project/docs"],
      }),
    );
    const runtime = new AgentRuntime(request.task_id, request.session_id);
    runtime.bindRequest(request);

    await adapter.execute(runtime, { send() {} }, request, "System prompt");

    expect(launches).toEqual([
      expect.objectContaining({
        runtime: "gemini",
        command: "gemini",
        commandArgs: [
          "-p",
          "System prompt\n\nImplement the requested bridge change.",
          "--output-format",
          "stream-json",
          "--approval-mode=auto_edit",
          "--model",
          "gemini-2.5-pro",
          "--include-directories",
          "D:/Project/shared,D:/Project/docs",
        ],
        stdinPayload: undefined,
      }),
    ]);
  });

  test("launches qoder through documented print mode and yolo flag instead of legacy aliases", async () => {
    const launches: Array<{
      runtime: string;
      command: string;
      commandArgs?: string[];
      stdinPayload?: string;
    }> = [];
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return command === "qodercli" ? "C:/mock/qodercli.exe" : null;
      },
      envLookup(name) {
        return name === "ANTHROPIC_API_KEY" ? "test-token" : undefined;
      },
      commandRuntimeRunner: async function* capture(params) {
        launches.push({
          runtime: params.runtime,
          command: params.command,
          commandArgs: params.commandArgs,
          stdinPayload: params.stdinPayload,
        });
      },
    });

    const { adapter, request } = await registry.resolveExecute(
      createRequest({
        runtime: "qoder",
        provider: "qoder",
        model: "ultimate",
        permission_mode: "bypassPermissions",
      }),
    );
    const runtime = new AgentRuntime(request.task_id, request.session_id);
    runtime.bindRequest(request);

    await adapter.execute(runtime, { send() {} }, request, "System prompt");

    expect(launches).toEqual([
      expect.objectContaining({
        runtime: "qoder",
        command: "qodercli",
        commandArgs: [
          "--print",
          "-p",
          "System prompt\n\nImplement the requested bridge change.",
          "--output-format",
          "stream-json",
          "--yolo",
          "-m",
          "ultimate",
          "-w",
          "D:/Project/AgentForge",
        ],
        stdinPayload: undefined,
      }),
    ]);
  });

  test("launches iflow through documented non-interactive prompt path and publishes sunset guidance", async () => {
    const launches: Array<{
      runtime: string;
      command: string;
      commandArgs?: string[];
      stdinPayload?: string;
    }> = [];
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        switch (command) {
          case "iflow":
            return "C:/mock/iflow.exe";
          default:
            return null;
        }
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "IFLOW_API_KEY":
            return "iflow-token";
          default:
            return undefined;
        }
      },
      now: () => Date.parse("2026-04-13T08:00:00+08:00"),
      commandRuntimeRunner: async function* capture(params) {
        launches.push({
          runtime: params.runtime,
          command: params.command,
          commandArgs: params.commandArgs,
          stdinPayload: params.stdinPayload,
        });
      },
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "iflow",
          available: true,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({
              code: "sunset_window",
              blocking: false,
            }),
          ]),
          launchContract: expect.objectContaining({
            promptTransport: "prompt_flag",
            outputMode: "text",
            supportedApprovalModes: ["default", "yolo"],
            additionalDirectories: true,
            envOverrides: false,
          }),
          lifecycle: expect.objectContaining({
            stage: "sunsetting",
            replacementRuntime: "qoder",
          }),
        }),
      ]),
    );

    const { adapter, request } = await registry.resolveExecute(
      createRequest({
        runtime: "iflow",
        provider: "iflow",
        model: "Qwen3-Coder",
        additional_directories: ["D:/Project/shared"],
      }),
    );
    const runtime = new AgentRuntime(request.task_id, request.session_id);
    runtime.bindRequest(request);

    await adapter.execute(runtime, { send() {} }, request, "System prompt");

    expect(launches).toEqual([
      expect.objectContaining({
        runtime: "iflow",
        command: "iflow",
        commandArgs: [
          "--prompt",
          "System prompt\n\nImplement the requested bridge change.",
          "--model",
          "Qwen3-Coder",
          "--add-dir",
          "D:/Project/shared",
        ],
        stdinPayload: undefined,
      }),
    ]);
  });

  test("rejects iflow after the published sunset date", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        switch (command) {
          case "iflow":
            return "C:/mock/iflow.exe";
          default:
            return null;
        }
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "IFLOW_API_KEY":
            return "iflow-token";
          default:
            return undefined;
        }
      },
      now: () => Date.parse("2026-04-18T08:00:00+08:00"),
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "iflow",
          available: false,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({
              code: "runtime_sunset",
              blocking: true,
            }),
          ]),
          lifecycle: expect.objectContaining({
            stage: "sunset",
          }),
        }),
      ]),
    );

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "iflow",
          provider: "iflow",
          model: "Qwen3-Coder",
        }),
      ),
    ).rejects.toThrow("Runtime iflow has reached its published sunset date");
  });

  test("surfaces degraded OpenCode catalog diagnostics when discovery helpers fail", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
        getAgents() {
          throw new Error("agent discovery offline");
        },
        getSkills() {
          throw new Error("skill discovery offline");
        },
        getProviderCatalog() {
          throw new Error("provider catalog offline");
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "opencode",
          available: true,
          diagnostics: expect.arrayContaining([
            expect.objectContaining({
              code: "catalog_agents_unavailable",
              blocking: false,
            }),
            expect.objectContaining({
              code: "catalog_skills_unavailable",
              blocking: false,
            }),
            expect.objectContaining({
              code: "catalog_providers_unavailable",
              blocking: false,
            }),
          ]),
        }),
      ]),
    );
  });

  test("publishes provider-auth guidance when OpenCode providers require Bridge-startable auth", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
        getProviderCatalog() {
          return Promise.resolve({
            availableProviders: ["anthropic"],
            connectedProviders: [],
            defaultModels: {
              anthropic: "claude-sonnet-4-5",
            },
            providerModels: {
              anthropic: ["claude-sonnet-4-5"],
            },
            authMethods: {
              anthropic: ["oauth"],
            },
          });
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "opencode",
          interactionCapabilities: expect.objectContaining({
            approval: expect.objectContaining({
              provider_auth: expect.objectContaining({
                state: "degraded",
                reasonCode: "provider_auth_required",
                message: expect.stringContaining("/bridge/opencode/provider-auth"),
              }),
            }),
          }),
        }),
      ]),
    );
  });

  test("publishes OpenCode parity input and rollback capability truth from the transport surface", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
        getExecuteCapabilities() {
          return {
            attachments: true,
            env: true,
            web_search: true,
            rollback: true,
          };
        },
      } as never,
    });

    const catalog = await registry.getCatalog();
    expect(catalog.runtimes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "opencode",
          interactionCapabilities: expect.objectContaining({
            inputs: expect.objectContaining({
              attachments: expect.objectContaining({ state: "supported" }),
              env: expect.objectContaining({ state: "supported" }),
              web_search: expect.objectContaining({ state: "supported" }),
            }),
            lifecycle: expect.objectContaining({
              rollback: expect.objectContaining({ state: "supported" }),
            }),
          }),
        }),
      ]),
    );
  });

  test("rejects OpenCode parity-sensitive execute inputs when the transport cannot honor them truthfully", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          case "OPENCODE_RUNTIME_MODEL":
            return "opencode-default";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
        getExecuteCapabilities() {
          return {
            attachments: false,
            env: false,
            web_search: false,
            rollback: false,
          };
        },
      } as never,
    });

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "opencode",
          provider: "opencode",
          attachments: [{ type: "image", path: "D:/tmp/screen.png" }],
        }),
      ),
    ).rejects.toThrow("Runtime opencode cannot honor attachments");

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "opencode",
          provider: "opencode",
          env: {
            FEATURE_FLAG: "enabled",
          },
        }),
      ),
    ).rejects.toThrow("Runtime opencode cannot honor env");

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "opencode",
          provider: "opencode",
          web_search: true,
        }),
      ),
    ).rejects.toThrow("Runtime opencode cannot honor web_search");
  });

  test("rejects explicit runtime/provider combinations that are incompatible", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in using an API key",
        };
      },
      envLookup() {
        return "test-token";
      },
    });

    await expect(
      registry.resolveExecute(
        createRequest({ runtime: "codex", provider: "anthropic", model: "gpt-5-codex" }),
      ),
    ).rejects.toThrow("Runtime codex is incompatible with provider anthropic");
  });

  test("rejects unknown runtime hints and missing runtime executables", async () => {
    const registry = createRuntimeRegistry({
      executableLookup() {
        return null;
      },
      envLookup() {
        return "test-token";
      },
    });

    await expect(
      registry.resolveExecute(createRequest({ runtime: "made_up_runtime" as never })),
    ).rejects.toThrow("Unknown runtime: made_up_runtime");
    await expect(registry.resolveExecute(createRequest({ runtime: "codex" }))).rejects.toThrow(
      "Executable not found for runtime codex",
    );
  });

  test("rejects claude_code when the required credential is missing", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup() {
        return undefined;
      },
    });

    await expect(registry.resolveExecute(createRequest({ runtime: "claude_code" }))).rejects.toThrow(
      "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
    );
  });

  test("rejects callback-dependent Claude requests that omit a callback target", async () => {
    const registry = createRuntimeRegistry({
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      envLookup(name) {
        return name === "ANTHROPIC_API_KEY" ? "test-token" : undefined;
      },
    });

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "claude_code",
          hooks_config: {
            hooks: [{ hook: "PreToolUse" }],
          },
        }),
      ),
    ).rejects.toThrow("hook_callback_url is required when Claude callback interactions are enabled");

    await expect(
      registry.resolveExecute(
        createRequest({
          runtime: "claude_code",
          tool_permission_callback: true,
        }),
      ),
    ).rejects.toThrow("hook_callback_url is required when Claude callback interactions are enabled");
  });

  test("dispatches supported advanced operations and throws typed errors for unsupported ones", async () => {
    const runtime = new AgentRuntime("task-opencode", "session-opencode");
    runtime.bindRequest(
      createRequest({
        runtime: "opencode",
        provider: "opencode",
        model: "opencode-default",
      }),
    );
    runtime.continuity = {
      runtime: "opencode",
      resume_ready: true,
      captured_at: 100,
      upstream_session_id: "opencode-session-123",
      fork_available: true,
      revert_message_ids: ["message-1"],
    };

    const registry = createRuntimeRegistry({
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({
            ok: true,
            diagnostics: [],
          });
        },
      } as never,
      advancedOperations: {
        opencode: {
          async getDiff() {
            return [{ path: "src/index.ts", diff: "@@ -1 +1 @@" }];
          },
          async fork(_runtime, params) {
            return {
              continuity: {
                runtime: "opencode" as const,
                resume_ready: true,
                captured_at: 200,
                upstream_session_id: params.message_id
                  ? `fork:${params.message_id}`
                  : "forked-session",
                fork_available: true,
                revert_message_ids: [],
              },
            };
          },
        },
      },
    });

    await expect(registry.getDiff(runtime)).resolves.toEqual([
      { path: "src/index.ts", diff: "@@ -1 +1 @@" },
    ]);
    await expect(
      registry.fork(runtime, {
        message_id: "message-1",
      }),
    ).resolves.toMatchObject({
      continuity: {
        upstream_session_id: "fork:message-1",
      },
    });
    await expect(registry.interrupt(runtime)).rejects.toBeInstanceOf(UnsupportedOperationError);
    await expect(registry.interrupt(runtime)).rejects.toMatchObject({
      operation: "interrupt",
      runtime: "opencode",
    });
  });

  test("dispatches Claude query control operations through the active query handle", async () => {
    const calls: Array<{ kind: string; payload?: unknown }> = [];
    const runtime = new AgentRuntime("task-claude", "session-claude");
    runtime.bindRequest(
      createRequest({
        task_id: "task-claude",
        session_id: "session-claude",
        runtime: "claude_code",
        provider: "anthropic",
        model: "claude-sonnet-4-5",
      }),
    );
    runtime.continuity = {
      runtime: "claude_code",
      resume_ready: true,
      captured_at: 100,
      session_handle: "claude-session-1",
      checkpoint_id: "assistant-uuid-1",
      query_ref: "task-claude",
      fork_available: true,
    };
    runtime.claudeQuery = {
      interrupt: async () => {
        calls.push({ kind: "interrupt" });
      },
      setModel: async (model?: string) => {
        calls.push({ kind: "setModel", payload: model });
      },
      setMaxThinkingTokens: async (tokens: number | null) => {
        calls.push({ kind: "setMaxThinkingTokens", payload: tokens });
      },
      rewindFiles: async (messageId: string) => {
        calls.push({ kind: "rewindFiles", payload: messageId });
        return { canRewind: true };
      },
      mcpServerStatus: async () => [{ name: "github", healthy: true }],
    };

    const forkCalls: Array<{ sessionId: string; upToMessageId?: string; dir?: string }> = [];
    const registry = createRuntimeRegistry({
      envLookup(name) {
        return name === "ANTHROPIC_API_KEY" ? "test-token" : undefined;
      },
      forkSessionRunner: async (sessionId, options) => {
        forkCalls.push({
          sessionId,
          upToMessageId: options?.upToMessageId,
          dir: options?.dir,
        });
        return { sessionId: "claude-session-forked" };
      },
    });

    await registry.interrupt(runtime);
    await registry.setModel(runtime, { model: "claude-haiku-4-5" });
    await registry.setThinkingBudget(runtime, { max_thinking_tokens: 2_048 });
    await registry.rollback(runtime, { checkpoint_id: "assistant-uuid-2" });
    await expect(registry.getMcpServerStatus(runtime)).resolves.toEqual([
      { name: "github", healthy: true },
    ]);
    await expect(
      registry.fork(runtime, {
        message_id: "assistant-uuid-2",
      }),
    ).resolves.toMatchObject({
      continuity: {
        runtime: "claude_code",
        session_handle: "claude-session-forked",
        checkpoint_id: "assistant-uuid-2",
        fork_available: true,
      },
    });

    expect(calls).toEqual([
      { kind: "interrupt" },
      { kind: "setModel", payload: "claude-haiku-4-5" },
      { kind: "setMaxThinkingTokens", payload: 2_048 },
      { kind: "rewindFiles", payload: "assistant-uuid-2" },
    ]);
    expect(forkCalls).toEqual([
      {
        sessionId: "claude-session-1",
        upToMessageId: "assistant-uuid-2",
        dir: "D:/Project/AgentForge",
      },
    ]);
  });

  test("dispatches Codex fork through an injected codex fork runner", async () => {
    const runtime = new AgentRuntime("task-codex-fork", "session-codex-fork");
    runtime.bindRequest(
      createRequest({
        task_id: "task-codex-fork",
        session_id: "session-codex-fork",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
      }),
    );
    runtime.continuity = {
      runtime: "codex",
      resume_ready: true,
      captured_at: 100,
      thread_id: "thread-codex-source",
      fork_available: true,
      rollback_turns: 2,
    };

    const forkCalls: Array<{ command: string; threadId: string; cwd?: string }> = [];
    const registry = createRuntimeRegistry({
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          default:
            return undefined;
        }
      },
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in",
        };
      },
      codexForkRunner: async ({ command, threadId, cwd }) => {
        forkCalls.push({ command, threadId, cwd });
        return { threadId: "thread-codex-forked" };
      },
      now: () => 555,
    });

    await expect(
      registry.fork(runtime, {}),
    ).resolves.toMatchObject({
      continuity: {
        runtime: "codex",
        thread_id: "thread-codex-forked",
        fork_available: true,
        rollback_turns: 0,
        captured_at: 555,
      },
    });
    expect(forkCalls).toEqual([
      {
        command: "codex",
        threadId: "thread-codex-source",
        cwd: "D:/Project/AgentForge",
      },
    ]);
  });

  test("dispatches Codex rollback through an injected rollback runner and refreshes continuity", async () => {
    const runtime = new AgentRuntime("task-codex-rollback", "session-codex-rollback");
    runtime.bindRequest(
      createRequest({
        task_id: "task-codex-rollback",
        session_id: "session-codex-rollback",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
      }),
    );
    runtime.continuity = {
      runtime: "codex",
      resume_ready: true,
      captured_at: 100,
      thread_id: "thread-codex-source",
      fork_available: true,
      rollback_turns: 3,
    };

    const rollbackCalls: Array<{ command: string; threadId: string; cwd?: string; turns?: number; checkpointId?: string }> = [];
    const registry = createRuntimeRegistry(({
      envLookup(name: string) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          default:
            return undefined;
        }
      },
      executableLookup(command: string) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in",
        };
      },
      codexRollbackRunner: async ({ command, threadId, cwd, turns, checkpointId }: {
        command: string;
        threadId: string;
        cwd?: string;
        turns?: number;
        checkpointId?: string;
      }) => {
        rollbackCalls.push({ command, threadId, cwd, turns, checkpointId });
        return {
          threadId,
          rollbackTurns: 2,
        };
      },
      now: () => 777,
    }) as unknown as Parameters<typeof createRuntimeRegistry>[0]);

    await registry.rollback(runtime, { turns: 1 });

    expect(rollbackCalls).toEqual([
      {
        command: "codex",
        threadId: "thread-codex-source",
        cwd: "D:/Project/AgentForge",
        turns: 1,
        checkpointId: undefined,
      },
    ]);
    expect(runtime.continuity).toMatchObject({
      runtime: "codex",
      thread_id: "thread-codex-source",
      rollback_turns: 2,
      captured_at: 777,
      resume_ready: true,
    });
  });

  test("default Codex fork runner detects the newly materialized rollout file", async () => {
    const sourceThreadId = "019d3c64-0f9a-7792-91a4-9f2dc058ddf8";
    const forkedThreadId = "019d3c65-1111-7222-8333-aaaaaaaaaaaa";
    const existingRollouts = [
      `C:/Users/test/.codex/sessions/2026/03/30/rollout-2026-03-30T09-38-03-${sourceThreadId}.jsonl`,
    ];
    const forkedRollout =
      `C:/Users/test/.codex/sessions/2026/03/30/rollout-2026-03-30T09-39-03-${forkedThreadId}.jsonl`;

    const spawnCalls: Array<{ cmd: string[]; cwd?: string }> = [];
    const killCalls: string[] = [];
    let listCalls = 0;
    let resolveExit: ((code: number) => void) | undefined;

    const result = await defaultCodexForkRunner(
      {
        command: "codex",
        threadId: sourceThreadId,
        cwd: "D:/Project/AgentForge",
      },
      {
        getSessionsRoot: () => "C:/Users/test/.codex/sessions",
        listRolloutFiles: () => {
          listCalls += 1;
          return listCalls >= 2 ? [...existingRollouts, forkedRollout] : existingRollouts;
        },
        readRolloutMeta: (filePath) => {
          if (filePath === forkedRollout) {
            return {
              threadId: forkedThreadId,
              forkedFromId: sourceThreadId,
            };
          }

          return {
            threadId: sourceThreadId,
          };
        },
        spawn: ({ cmd, cwd }) => {
          spawnCalls.push({ cmd, cwd });
          return {
            stdout: null,
            stderr: null,
            kill() {
              killCalls.push("killed");
              resolveExit?.(0);
            },
            exited: new Promise<number>((resolve) => {
              resolveExit = resolve;
            }),
          } as never;
        },
        sleep: async () => {},
        timeoutMs: 5,
      },
    );

    expect(result).toEqual({ threadId: forkedThreadId });
    expect(spawnCalls).toEqual([
      {
        cmd: [
          "codex",
          "fork",
          sourceThreadId,
          "-C",
          "D:/Project/AgentForge",
          "--no-alt-screen",
        ],
        cwd: "D:/Project/AgentForge",
      },
    ]);
    expect(killCalls).toEqual(["killed"]);
  });
});
