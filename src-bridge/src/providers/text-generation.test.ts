import { describe, expect, test } from "bun:test";
import { runTextGeneration } from "./text-generation.js";
import { resolveProviderSelection } from "./registry.js";

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
});
