import { describe, expect, test } from "bun:test";
import { RuntimePoolManager } from "./pool-manager.js";

describe("RuntimePoolManager", () => {
  test("acquires, lists, and releases runtimes", () => {
    const pool = new RuntimePoolManager(2);
    const runtime = pool.acquire("task-123", "session-123");

    expect(pool.get("task-123")).toBe(runtime);
    expect(pool.listActive()).toHaveLength(1);
    expect(pool.stats()).toEqual({ active: 1, max: 2 });

    pool.release("task-123");

    expect(pool.get("task-123")).toBeUndefined();
    expect(pool.listActive()).toHaveLength(0);
  });

  test("rejects duplicate runtimes and over-capacity acquisition", () => {
    const pool = new RuntimePoolManager(1);

    pool.acquire("task-123", "session-123");

    expect(() => pool.acquire("task-123", "session-456")).toThrow(
      "Runtime already exists for task task-123",
    );
    expect(() => pool.acquire("task-999", "session-999")).toThrow(
      "Pool at capacity (1). Cannot acquire runtime for task task-999",
    );
  });
});
