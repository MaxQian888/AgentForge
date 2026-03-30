import { afterEach, describe, expect, mock, test } from "bun:test";
import { handleGenerate } from "./generate.js";
import { resolveProviderSelection } from "../providers/registry.js";

let dynamicImportCounter = 0;

afterEach(() => {
  mock.restore();
  mock.clearAllMocks();
});

async function importGenerateModule(tag: string) {
  dynamicImportCounter += 1;
  return import(`./generate.ts?${tag}-${dynamicImportCounter}`);
}

describe("handleGenerate", () => {
  test("forwards prompt options and resolved provider selection to the executor", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "google",
      model: "gemini-2.5-flash",
    });
    let invocation:
      | {
          prompt: string;
          systemPrompt?: string;
          provider: string;
          maxTokens?: number;
          temperature?: number;
        }
      | undefined;

    const result = await handleGenerate(
      {
        prompt: "Summarize the latest task activity.",
        system_prompt: "Respond in one short sentence.",
        max_tokens: 64,
        temperature: 0,
      },
      provider,
      async (params) => {
        invocation = {
          prompt: params.prompt,
          systemPrompt: params.systemPrompt,
          provider: `${params.provider.provider}:${params.provider.model}`,
          maxTokens: params.maxTokens,
          temperature: params.temperature,
        };
        return {
          text: "Task activity summarized.",
          usage: {
            input_tokens: 12,
            output_tokens: 6,
          },
        };
      },
    );

    expect(invocation).toEqual({
      prompt: "Summarize the latest task activity.",
      systemPrompt: "Respond in one short sentence.",
      provider: "google:gemini-2.5-flash",
      maxTokens: 64,
      temperature: 0,
    });
    expect(result).toEqual({
      text: "Task activity summarized.",
      usage: {
        input_tokens: 12,
        output_tokens: 6,
      },
    });
  });

  test("fails before model execution when provider credentials are missing", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    const previous = process.env.OPENAI_API_KEY;
    delete process.env.OPENAI_API_KEY;

    try {
      await expect(
        handleGenerate(
          {
            prompt: "Write a concise summary.",
          },
          provider,
        ),
      ).rejects.toThrow("Provider openai is missing credentials: OPENAI_API_KEY");
    } finally {
      if (previous === undefined) {
        delete process.env.OPENAI_API_KEY;
      } else {
        process.env.OPENAI_API_KEY = previous;
      }
    }
  });

  test("uses the default executor with the OpenAI model factory and normalized prompt/completion usage", async () => {
    const calls: Array<Record<string, unknown>> = [];
    const previous = process.env.OPENAI_API_KEY;
    process.env.OPENAI_API_KEY = "test-openai-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      generateText: async (params: Record<string, unknown>) => {
        calls.push(params);
        return {
          text: "OpenAI summary.",
          usage: {
            promptTokens: 21,
            completionTokens: 9,
          },
        };
      },
    }));

    try {
      const { handleGenerate: runDefaultGenerate } = await importGenerateModule("openai-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "openai",
        model: "gpt-5",
      });

      await expect(
        runDefaultGenerate(
          {
            prompt: "Summarize the bridge state.",
            system_prompt: "One sentence only.",
            max_tokens: 64,
            temperature: 0.5,
          },
          provider,
        ),
      ).resolves.toEqual({
        text: "OpenAI summary.",
        usage: {
          input_tokens: 21,
          output_tokens: 9,
        },
      });

      expect(calls).toEqual([
        {
          model: { provider: "openai", model: "gpt-5" },
          prompt: "Summarize the bridge state.",
          system: "One sentence only.",
          maxTokens: 64,
          temperature: 0.5,
        },
      ]);
    } finally {
      if (previous === undefined) {
        delete process.env.OPENAI_API_KEY;
      } else {
        process.env.OPENAI_API_KEY = previous;
      }
    }
  });

  test("uses the default executor with the Google model factory and legacy usage fields", async () => {
    const calls: Array<Record<string, unknown>> = [];
    const previous = process.env.GOOGLE_GENERATIVE_AI_API_KEY;
    process.env.GOOGLE_GENERATIVE_AI_API_KEY = "test-google-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      generateText: async (params: Record<string, unknown>) => {
        calls.push(params);
        return {
          text: "Google summary.",
          usage: {
            input_tokens: 13,
            output_tokens: 7,
          },
        };
      },
    }));

    try {
      const { handleGenerate: runDefaultGenerate } = await importGenerateModule("google-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "google",
        model: "gemini-2.5-flash",
      });

      await expect(
        runDefaultGenerate(
          {
            prompt: "List the latest bridge events.",
          },
          provider,
        ),
      ).resolves.toEqual({
        text: "Google summary.",
        usage: {
          input_tokens: 13,
          output_tokens: 7,
        },
      });

      expect(calls).toEqual([
        {
          model: { provider: "google", model: "gemini-2.5-flash" },
          prompt: "List the latest bridge events.",
        },
      ]);
    } finally {
      if (previous === undefined) {
        delete process.env.GOOGLE_GENERATIVE_AI_API_KEY;
      } else {
        process.env.GOOGLE_GENERATIVE_AI_API_KEY = previous;
      }
    }
  });

  test("uses the default executor with the Anthropic model factory and zeroes missing usage", async () => {
    const calls: Array<Record<string, unknown>> = [];
    const previousApiKey = process.env.ANTHROPIC_API_KEY;
    process.env.ANTHROPIC_API_KEY = "test-anthropic-key";

    mock.module("@ai-sdk/anthropic", () => ({ anthropic: (model: string) => ({ provider: "anthropic", model }) }));
    mock.module("@ai-sdk/google", () => ({ google: (model: string) => ({ provider: "google", model }) }));
    mock.module("@ai-sdk/openai", () => ({ openai: (model: string) => ({ provider: "openai", model }) }));
    mock.module("ai", () => ({
      generateText: async (params: Record<string, unknown>) => {
        calls.push(params);
        return {
          text: "Anthropic summary.",
        };
      },
    }));

    try {
      const { handleGenerate: runDefaultGenerate } = await importGenerateModule("anthropic-default");
      const provider = resolveProviderSelection("text_generation", {
        provider: "anthropic",
        model: "claude-haiku-4-5",
      });

      await expect(
        runDefaultGenerate(
          {
            prompt: "Generate a short summary.",
            temperature: 0,
          },
          provider,
        ),
      ).resolves.toEqual({
        text: "Anthropic summary.",
        usage: {
          input_tokens: 0,
          output_tokens: 0,
        },
      });

      expect(calls).toEqual([
        {
          model: { provider: "anthropic", model: "claude-haiku-4-5" },
          prompt: "Generate a short summary.",
          temperature: 0,
        },
      ]);
    } finally {
      if (previousApiKey === undefined) {
        delete process.env.ANTHROPIC_API_KEY;
      } else {
        process.env.ANTHROPIC_API_KEY = previousApiKey;
      }
    }
  });
});
