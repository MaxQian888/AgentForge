import { describe, expect, test } from "bun:test";
import { handleClassifyIntent } from "./classify-intent.js";
import { resolveProviderSelection } from "../providers/registry.js";

describe("handleClassifyIntent", () => {
  test("builds the classifier prompt and forwards the resolved provider to the executor", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    let receivedPrompt = "";
    let receivedProvider = "";

    const result = await handleClassifyIntent(
      {
        text: "帮我创建一个修复 bridge 覆盖率的任务",
        user_id: "user-123",
        project_id: "project-456",
      },
      provider,
      async ({ prompt, provider: resolved }) => {
        receivedPrompt = prompt;
        receivedProvider = `${resolved.provider}:${resolved.model}`;
        return {
          intent: "create_task",
          command: "/task create",
          args: "修复 bridge 覆盖率",
          confidence: 0.92,
          reply: "马上创建。",
        };
      },
    );

    expect(receivedPrompt).toContain(
      "User message: 帮我创建一个修复 bridge 覆盖率的任务",
    );
    expect(receivedPrompt).toContain("/task create <title>");
    expect(receivedProvider).toBe("openai:gpt-5");
    expect(result.intent).toBe("create_task");
  });

  test("includes candidates and context history in the classifier prompt when provided", async () => {
    const provider = resolveProviderSelection("text_generation", {
      provider: "openai",
      model: "gpt-5",
    });
    let receivedPrompt = "";

    await handleClassifyIntent(
      {
        text: "@AgentForge 帮我看看帮助",
        user_id: "user-123",
        project_id: "project-456",
        candidates: ["help", "task_list", "sprint_status"],
        context: {
          history: ["上一条消息", "再上一条消息"],
          thread_id: "thread-1",
        },
      } as never,
      provider,
      async ({ prompt }) => {
        receivedPrompt = prompt;
        return {
          intent: "help",
          command: "/help",
          args: "",
          confidence: 0.88,
        };
      },
    );

    expect(receivedPrompt).toContain("help");
    expect(receivedPrompt).toContain("task_list");
    expect(receivedPrompt).toContain("thread-1");
    expect(receivedPrompt).toContain("上一条消息");
  });

  test("falls back to an unknown response when executor output is invalid", async () => {
    const provider = resolveProviderSelection("text_generation");

    const result = await handleClassifyIntent(
      {
        text: "这句话无法正确分类",
        user_id: "",
        project_id: "",
      },
      provider,
      async () =>
        ({
          intent: "create_task",
          command: "/task create",
          args: "invalid",
          confidence: 10,
        }) as never,
    );

    expect(result).toEqual({
      intent: "unknown",
      command: "",
      args: "",
      confidence: 0,
      reply: "无法理解您的请求。请使用 /help 查看可用命令。",
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
        handleClassifyIntent(
          {
            text: "列出当前任务",
            user_id: "",
            project_id: "",
          },
          provider,
        ),
      ).rejects.toThrow(
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
});
