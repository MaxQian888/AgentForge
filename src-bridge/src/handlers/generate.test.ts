import { describe, expect, test } from "bun:test";
import { handleGenerate } from "./generate.js";
import { resolveProviderSelection } from "../providers/registry.js";

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
});
