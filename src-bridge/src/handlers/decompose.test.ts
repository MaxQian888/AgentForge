import { describe, expect, test } from "bun:test";
import {
  buildDecompositionPrompt,
  createTextGenerationDecomposeExecutor,
  handleDecompose,
} from "./decompose.js";
import type { DecomposeTaskRequest } from "../types.js";
import { resolveProviderSelection } from "../providers/registry.js";

const request: DecomposeTaskRequest = {
  task_id: "task-123",
  title: "Bridge task decomposition",
  description: "Split the bridge feature into focused deliverables.",
  priority: "high",
};

describe("decompose handler", () => {
  test("builds a prompt with the request context", () => {
    const prompt = buildDecompositionPrompt(request);

    expect(prompt).toContain("Task ID: task-123");
    expect(prompt).toContain("Title: Bridge task decomposition");
    expect(prompt).toContain("Priority: high");
    expect(prompt).toContain("Split the bridge feature into focused deliverables.");
    expect(prompt).toContain("executionMode");
  });

  test("includes optional decomposition context in the generated prompt", () => {
    const prompt = buildDecompositionPrompt({
      ...request,
      context: {
        relevantFiles: ["src-go/internal/server/routes.go"],
        waveMode: true,
      },
    });

    expect(prompt).toContain("Additional Context");
    expect(prompt).toContain("\"relevantFiles\":[\"src-go/internal/server/routes.go\"]");
    expect(prompt).toContain("\"waveMode\":true");
  });

  test("passes the resolved provider context to the executor", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    let receivedProvider = "";
    const result = await handleDecompose(request, provider, async ({ provider: resolved }) => {
      receivedProvider = `${resolved.provider}:${resolved.model}`;
      return {
        summary: 'Decomposed "Bridge task decomposition" into focused delivery steps.',
        subtasks: [
          {
            title: "Clarify task contract",
            description: "Review the current interfaces and constraints.",
            priority: "high",
            executionMode: "human",
          },
        ],
      };
    });

    expect(receivedProvider).toBe("openai:gpt-5");
    expect(result.summary).toContain('Decomposed "Bridge task decomposition"');
    expect(result.subtasks).toHaveLength(1);
  });

  test("rejects invalid executor output", async () => {
    const provider = resolveProviderSelection("text_generation");
    await expect(
      handleDecompose(request, provider, async () => ({
        summary: "",
        subtasks: [],
      })),
    ).rejects.toThrow("Invalid decomposition output");
  });

  test("uses the default text-generation executor to return structured output", async () => {
    process.env.OPENAI_API_KEY = "test-openai-key";
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    const executor = createTextGenerationDecomposeExecutor(async () => ({
      output: {
        summary: "Split the work into bridge and validation steps.",
        subtasks: [
          {
            title: "Add provider registry",
            description: "Introduce shared provider capability metadata.",
            priority: "high",
            executionMode: "agent",
          },
        ],
      },
    }));

    const result = await handleDecompose(request, provider, executor);

    expect(result).toEqual({
      summary: "Split the work into bridge and validation steps.",
      subtasks: [
        {
          title: "Add provider registry",
          description: "Introduce shared provider capability metadata.",
          priority: "high",
          executionMode: "agent",
        },
      ],
    });
    delete process.env.OPENAI_API_KEY;
  });

  test("fails cleanly when provider structured output is invalid", async () => {
    process.env.OPENAI_API_KEY = "test-openai-key";
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    const executor = createTextGenerationDecomposeExecutor(async () => ({
      output: {
        summary: "",
        subtasks: [],
      } as never,
    }));

    await expect(handleDecompose(request, provider, executor)).rejects.toThrow(
      "Invalid decomposition output",
    );
    delete process.env.OPENAI_API_KEY;
  });

  test("fails when the resolved provider is missing credentials", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });

    delete process.env.OPENAI_API_KEY;

    await expect(handleDecompose(request, provider)).rejects.toThrow(
      "Provider openai is missing credentials: OPENAI_API_KEY",
    );
  });

  test("accepts Anthropic auth tokens as a valid credential source", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
    });
    delete process.env.ANTHROPIC_API_KEY;
    process.env.ANTHROPIC_AUTH_TOKEN = "test-auth-token";

    const result = await handleDecompose(
      request,
      provider,
      createTextGenerationDecomposeExecutor(async () => ({
        output: {
          summary: "Use auth token credentials successfully.",
          subtasks: [
            {
              title: "Authenticate Anthropic",
              description: "Allow auth token based configuration.",
              priority: "high",
              executionMode: "agent",
            },
          ],
        },
      })),
    );

    expect(result.summary).toBe("Use auth token credentials successfully.");
    delete process.env.ANTHROPIC_AUTH_TOKEN;
  });
});
