import { describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { SessionManager } from "./manager.js";

describe("SessionManager", () => {
  test("saves, restores, lists, and deletes session snapshots", () => {
    const manager = new SessionManager();
    const snapshot = {
      task_id: "task-123",
      session_id: "session-123",
      status: "running",
      turn_number: 4,
      spent_usd: 0.12,
      created_at: 100,
      updated_at: 200,
    };

    manager.save("task-123", snapshot);
    const restored = manager.restore("task-123");

    expect(restored).toEqual(snapshot);
    expect(restored).not.toBe(snapshot);
    expect(manager.list()).toEqual([snapshot]);

    manager.delete("task-123");

    expect(manager.restore("task-123")).toBeNull();
    expect(manager.list()).toEqual([]);
  });

  test("persists snapshots to disk and reloads them for session resume", () => {
    const baseDir = mkdtempSync(join(tmpdir(), "agentforge-session-manager-"));

    try {
      const manager = new SessionManager({ baseDir });
      const snapshot = {
        task_id: "task-456",
        session_id: "session-456",
        status: "paused",
        turn_number: 7,
        spent_usd: 0.42,
        created_at: 100,
        updated_at: 200,
        request: {
          task_id: "task-456",
          session_id: "session-456",
          prompt: "Resume the task",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-456",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "codex" as const,
        },
      };

      manager.save("task-456", snapshot);

      const reloaded = new SessionManager({ baseDir });

      expect(reloaded.restore("task-456")).toEqual(snapshot);
      expect(reloaded.list()).toEqual([snapshot]);
    } finally {
      rmSync(baseDir, { force: true, recursive: true });
    }
  });
});
