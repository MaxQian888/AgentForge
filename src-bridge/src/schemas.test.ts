import { describe, expect, test } from "bun:test";
import {
  CancelRequestSchema,
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
