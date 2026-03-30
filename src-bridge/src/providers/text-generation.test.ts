import { afterEach, describe, expect, mock, test } from "bun:test";
import { runTextGeneration } from "./text-generation.js";
import { resolveProviderSelection } from "./registry.js";

let dynamicImportCounter = 0;

afterEach(() => {
  mock.restore();
  mock.clearAllMocks();
});

async function importDefaultRunner(tag: string) {
  dynamicImportCounter += 1;
  return import(`./text-generation.ts?${tag}-${dynamicImportCounter}`);
}

describe("runTextGeneration", () => {
  test("forwards the resolved provider and prompt to the injected runner", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "google",
      model: "gemini-2.5-flash",
    });
    const previous = process.env.GOOGLE_GENERATIVE_AI_API_KEY;
    process.env.GOOGLE_GENERATIVE_AI_API_KEY = "test-google-key";
    let receivedProvider = "";
    let receivedPrompt = "";

    try {
      const result = await runTextGeneration(
        provider,
        "Decompose this bridge task.",
        async (params) => {
          receivedProvider = `${params.provider.provider}:${params.provider.model}`;
          receivedPrompt = params.prompt;
          return {
            output: {
              summary: "Split the work.",
              subtasks: [
                {
                  title: "Inspect the runtime registry",
                  description: "Verify runtime defaults and diagnostics.",
                  priority: "high",
                  executionMode: "agent",
                },
              ],
            },
          };
        },
      );

      expect(receivedProvider).toBe("google:gemini-2.5-flash");
      expect(receivedPrompt).toBe("Decompose this bridge task.");
      expect(result.output.summary).toBe("Split the work.");
    } finally {
      if (previous === undefined) {
        delete process.env.GOOGLE_GENERATIVE_AI_API_KEY;
      } else {
        process.env.GOOGLE_GENERATIVE_AI_API_KEY = previous;
      }
    }
  });

  test("fails before running when provider credentials are missing", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    const previous = process.env.OPENAI_API_KEY;
    delete process.env.OPENAI_API_KEY;

    try {
      await expect(runTextGeneration(provider, "Inspect coverage gaps.")).rejects.toThrow(
        "Provider openai is missing credentials: OPENAI_API_KEY",
      );
    } finally {
      if (previous === undefined) {
        delete process.env.OPENAI_API_KEY;
      } else {
        process.env.OPENAI_API_KEY = previous;
      }
    }
  });

  test("accepts Anthropic auth tokens as an alternative credential source", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
    });
    const previousApiKey = process.env.ANTHROPIC_API_KEY;
    const previousAuthToken = process.env.ANTHROPIC_AUTH_TOKEN;
    delete process.env.ANTHROPIC_API_KEY;
    process.env.ANTHROPIC_AUTH_TOKEN = "test-auth-token";

    try {
      const result = await runTextGeneration(
        provider,
        "Summarize the bridge task.",
        async () => ({
          output: {
            summary: "Auth token credentials worked.",
            subtasks: [
              {
                title: "Use the auth token",
                description: "Allow Anthropic auth token based execution.",
                priority: "high",
                executionMode: "agent",
              },
            ],
          },
        }),
      );

      expect(result.output.summary).toBe("Auth token credentials worked.");
    } finally {
      if (previousApiKey === undefined) {
        delete process.env.ANTHROPIC_API_KEY;
      } else {
        process.env.ANTHROPIC_API_KEY = previousApiKey;
      }

      if (previousAuthToken === undefined) {
        delete process.env.ANTHROPIC_AUTH_TOKEN;
      } else {
        process.env.ANTHROPIC_AUTH_TOKEN = previousAuthToken;
      }
    }
  });

  test("uses the default runner with the OpenAI model factory", async () => {
    const calls: Array<{ model: unknown; prompt: string; output: unknown }> = [];
    const previous = process.env.OPENAI_API_KEY;
    process.env.OPENAI_API_KEY = "test-openai-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      Output: {
        object: ({ schema }: { schema: unknown }) => ({ kind: "object", schema }),
      },
      generateText: async (params: { model: unknown; prompt: string; output: unknown }) => {
        calls.push(params);
        return {
          output: {
            summary: "OpenAI default runner worked.",
            subtasks: [],
          },
        };
      },
    }));

    try {
      const { runTextGeneration: runDefaultTextGeneration } = await importDefaultRunner("openai-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "openai",
        model: "gpt-5",
      });

      await expect(runDefaultTextGeneration(provider, "Summarize the bridge state.")).resolves.toEqual({
        output: {
          summary: "OpenAI default runner worked.",
          subtasks: [],
        },
      });
      expect(calls).toHaveLength(1);
      expect(calls[0]?.model).toEqual({ provider: "openai", model: "gpt-5" });
      expect(calls[0]?.prompt).toBe("Summarize the bridge state.");
      expect(calls[0]?.output).toMatchObject({ kind: "object" });
    } finally {
      if (previous === undefined) {
        delete process.env.OPENAI_API_KEY;
      } else {
        process.env.OPENAI_API_KEY = previous;
      }
    }
  });

  test("uses the default runner with the Google model factory", async () => {
    const calls: unknown[] = [];
    const previous = process.env.GOOGLE_GENERATIVE_AI_API_KEY;
    process.env.GOOGLE_GENERATIVE_AI_API_KEY = "test-google-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      Output: {
        object: ({ schema }: { schema: unknown }) => ({ kind: "object", schema }),
      },
      generateText: async (params: { model: unknown }) => {
        calls.push(params.model);
        return {
          output: {
            summary: "Google default runner worked.",
            subtasks: [],
          },
        };
      },
    }));

    try {
      const { runTextGeneration: runDefaultTextGeneration } = await importDefaultRunner("google-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "google",
        model: "gemini-2.5-flash",
      });

      await expect(runDefaultTextGeneration(provider, "Decompose this change.")).resolves.toEqual({
        output: {
          summary: "Google default runner worked.",
          subtasks: [],
        },
      });
      expect(calls).toEqual([{ provider: "google", model: "gemini-2.5-flash" }]);
    } finally {
      if (previous === undefined) {
        delete process.env.GOOGLE_GENERATIVE_AI_API_KEY;
      } else {
        process.env.GOOGLE_GENERATIVE_AI_API_KEY = previous;
      }
    }
  });

  test("uses the default runner with the Anthropic model factory", async () => {
    const calls: unknown[] = [];
    const previousApiKey = process.env.ANTHROPIC_API_KEY;
    process.env.ANTHROPIC_API_KEY = "test-anthropic-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      Output: {
        object: ({ schema }: { schema: unknown }) => ({ kind: "object", schema }),
      },
      generateText: async (params: { model: unknown }) => {
        calls.push(params.model);
        return {
          output: {
            summary: "Anthropic default runner worked.",
            subtasks: [],
          },
        };
      },
    }));

    try {
      const { runTextGeneration: runDefaultTextGeneration } = await importDefaultRunner("anthropic-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "anthropic",
        model: "claude-haiku-4-5",
      });

      await expect(runDefaultTextGeneration(provider, "Review the bridge diff.")).resolves.toEqual({
        output: {
          summary: "Anthropic default runner worked.",
          subtasks: [],
        },
      });
      expect(calls).toEqual([{ provider: "anthropic", model: "claude-haiku-4-5" }]);
    } finally {
      if (previousApiKey === undefined) {
        delete process.env.ANTHROPIC_API_KEY;
      } else {
        process.env.ANTHROPIC_API_KEY = previousApiKey;
      }
    }
  });
});
