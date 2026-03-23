import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "./agent-runtime.js";

describe("AgentRuntime", () => {
  test("starts with default state and exposes a status snapshot", () => {
    const runtime = new AgentRuntime("task-123", "session-123");

    expect(runtime.status).toBe("starting");
    expect(runtime.turnNumber).toBe(0);
    expect(runtime.spentUsd).toBe(0);
    expect(runtime.toStatus()).toMatchObject({
      task_id: "task-123",
      state: "starting",
      turn_number: 0,
      last_tool: "",
      spent_usd: 0,
    });
  });

  test("marks the runtime as failed when cancelled", () => {
    const runtime = new AgentRuntime("task-456", "session-456");

    runtime.cancel();

    expect(runtime.status).toBe("failed");
    expect(runtime.abortController.signal.aborted).toBe(true);
  });
});
