import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "./agent-runtime.js";

describe("AgentRuntime", () => {
  test("starts with default state and exposes a status snapshot", () => {
    const runtime = new AgentRuntime("task-123", "session-123");
    runtime.bindRequest({
      task_id: "task-123",
      session_id: "session-123",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      prompt: "Implement the runtime contract",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      system_prompt: "Base prompt",
      max_turns: 12,
      budget_usd: 5,
      allowed_tools: ["Read"],
      permission_mode: "default",
      team_id: "team-123",
      team_role: "planner",
      role_config: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Build reliable UI",
        backstory: "A frontend specialist.",
        system_prompt: "Stay consistent.",
        allowed_tools: ["Read"],
        max_budget_usd: 5,
        max_turns: 12,
        permission_mode: "default",
      },
    });

    expect(runtime.status).toBe("starting");
    expect(runtime.turnNumber).toBe(0);
    expect(runtime.spentUsd).toBe(0);
    expect(runtime.toStatus()).toMatchObject({
      task_id: "task-123",
      state: "starting",
      turn_number: 0,
      last_tool: "",
      spent_usd: 0,
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      role_id: "frontend-developer",
      team_id: "team-123",
      team_role: "planner",
    });
  });

  test("marks the runtime as cancelled when cancelled", () => {
    const runtime = new AgentRuntime("task-456", "session-456");

    runtime.cancel();

    expect(runtime.status).toBe("cancelled");
    expect(runtime.abortController.signal.aborted).toBe(true);
  });
});
