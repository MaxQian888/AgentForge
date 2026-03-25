import { describe, expect, test } from "bun:test";
import { RuntimePoolManager } from "./pool-manager.js";

describe("RuntimePoolManager", () => {
  test("acquires, lists, and releases runtimes", () => {
    const pool = new RuntimePoolManager(2);
    const runtime = pool.acquire("task-123", "session-123", "codex");

    expect(pool.get("task-123")).toBe(runtime);
    expect(pool.listActive()).toHaveLength(1);
    expect(pool.stats()).toMatchObject({
      active: 1,
      max: 2,
      warm_total: 0,
      warm_available: 0,
      warm_reuse_hits: 0,
      cold_starts: 1,
    });

    pool.release("task-123");

    expect(pool.get("task-123")).toBeUndefined();
    expect(pool.listActive()).toHaveLength(0);
    expect(pool.stats()).toMatchObject({
      warm_available: 1,
    });
  });

  test("rejects duplicate runtimes and over-capacity acquisition", () => {
    const pool = new RuntimePoolManager(1);

    pool.acquire("task-123", "session-123", "claude_code");

    expect(() => pool.acquire("task-123", "session-456")).toThrow(
      "Runtime already exists for task task-123",
    );
    expect(() => pool.acquire("task-999", "session-999", "claude_code")).toThrow(
      "Pool at capacity (1). Cannot acquire runtime for task task-999",
    );
  });

  test("reuses a released warm slot before counting another cold start", () => {
    const pool = new RuntimePoolManager(2);

    pool.acquire("task-1", "session-1", "opencode");
    pool.release("task-1");

    expect(pool.stats()).toMatchObject({
      warm_available: 1,
      cold_starts: 1,
      warm_reuse_hits: 0,
    });

    pool.acquire("task-2", "session-2", "opencode");

    expect(pool.stats()).toMatchObject({
      active: 1,
      warm_available: 0,
      cold_starts: 1,
      warm_reuse_hits: 1,
    });
  });
});
