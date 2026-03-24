import { describe, expect, test } from "bun:test";
import {
  CancelRequestSchema,
  DecomposeTaskRequestSchema,
  DeepReviewRequestSchema,
  ExecuteRequestSchema,
} from "./schemas.js";

describe("bridge request schemas", () => {
  test("applies defaults for execute requests", () => {
    const parsed = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
    });

    expect(parsed.system_prompt).toBe("");
    expect(parsed.max_turns).toBe(50);
    expect(parsed.allowed_tools).toEqual([]);
    expect(parsed.permission_mode).toBe("default");
  });

  test("preserves optional provider and model fields for execute and decompose requests", () => {
    const execute = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
      runtime: "opencode",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
    });

    const decompose = DecomposeTaskRequestSchema.parse({
      task_id: "task-123",
      title: "Split feature work",
      description: "Break this task down into focused subtasks.",
      priority: "high",
      provider: "openai",
      model: "gpt-5",
    });

    expect(execute.runtime).toBe("opencode");
    expect(execute.provider).toBe("anthropic");
    expect(execute.model).toBe("claude-sonnet-4-5");
    expect(decompose.provider).toBe("openai");
    expect(decompose.model).toBe("gpt-5");
  });

  test("rejects execute payloads with unknown runtime keys", () => {
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        runtime: "made_up_runtime",
      }).success,
    ).toBe(false);
  });

  test("accepts normalized role execution profiles and rejects raw YAML-shaped role payloads", () => {
    const parsed = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
      role_config: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Build reliable UI",
        backstory: "A frontend specialist",
        system_prompt: "You build safe UI.",
        allowed_tools: ["Read", "Edit"],
        max_budget_usd: 5,
        max_turns: 20,
        permission_mode: "default",
      },
    });

    expect(parsed.role_config?.role_id).toBe("frontend-developer");
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        role_config: {
          metadata: { id: "frontend-developer", name: "Frontend Developer" },
          security: { permission_mode: "default" },
        },
      }).success,
    ).toBe(false);
  });

  test("rejects invalid cancel and review payloads", () => {
    expect(CancelRequestSchema.safeParse({ task_id: "" }).success).toBe(false);
    expect(
      DeepReviewRequestSchema.safeParse({
        review_id: "review-123",
        task_id: "task-123",
        pr_url: "https://example.com/pr/123",
        pr_number: -1,
      }).success,
    ).toBe(false);
  });
});
