import { describe, expect, test } from "bun:test";
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
});
