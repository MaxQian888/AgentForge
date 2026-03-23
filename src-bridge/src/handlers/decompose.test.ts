import { describe, expect, test } from "bun:test";
import { buildDecompositionPrompt, handleDecompose } from "./decompose.js";
import type { DecomposeTaskRequest } from "../types.js";

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
  });

  test("simulates a structured decomposition when no executor is provided", async () => {
    const result = await handleDecompose(request);

    expect(result.summary).toContain('Decomposed "Bridge task decomposition"');
    expect(result.subtasks).toHaveLength(3);
    expect(result.subtasks.map((subtask) => subtask.priority)).toEqual([
      "high",
      "medium",
      "low",
    ]);
  });

  test("rejects invalid executor output", async () => {
    await expect(
      handleDecompose(request, async () => ({
        summary: "",
        subtasks: [],
      })),
    ).rejects.toThrow("Invalid decomposition output");
  });
});
