import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, type PooledEntryFactory } from "./connection-pool.js";

function stubFactory(): PooledEntryFactory {
  let counter = 0;
   
  return async (_adapterId) => {
    const id = ++counter;
    return {
      host: { shutdown: async () => {}, exited: Promise.resolve(0) } as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      conn: {} as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      caps: {} as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      clientDispatcher: { register: () => {}, unregister: () => {} } as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      sessions: new Set<string>(),
      restartPending: false,
      _id: id,
    } as any; // eslint-disable-line @typescript-eslint/no-explicit-any
  };
}

describe("AcpConnectionPool", () => {
  test("concurrent acquire → single spawn (mutex)", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({ logger: console as any, factory }); // eslint-disable-line @typescript-eslint/no-explicit-any
    const [a, b] = await Promise.all([
      pool.acquire("claude_code"),
      pool.acquire("claude_code"),
    ]);
    expect((a as any)._id).toBe((b as any)._id); // eslint-disable-line @typescript-eslint/no-explicit-any
  });

  test("release decrements ref count and schedules idle shutdown", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({
      logger: console as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      factory,
      idleMs: 20,
    });
    const entry = await pool.acquire("claude_code");
    entry.sessions.add("s1");
    await pool.release("claude_code", "s1");
    await new Promise((r) => setTimeout(r, 150));
    // After idle timeout, next acquire should spawn a new entry
    const again = await pool.acquire("claude_code");
    expect((again as any)._id).not.toBe((entry as any)._id); // eslint-disable-line @typescript-eslint/no-explicit-any
  });

  test("acquire on restartPending entry spawns fresh", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({ logger: console as any, factory }); // eslint-disable-line @typescript-eslint/no-explicit-any
    const first = await pool.acquire("claude_code");
    (first as any).restartPending = true; // eslint-disable-line @typescript-eslint/no-explicit-any
    const second = await pool.acquire("claude_code");
    expect((second as any)._id).not.toBe((first as any)._id); // eslint-disable-line @typescript-eslint/no-explicit-any
  });

  test("acquire on already-exited host spawns fresh", async () => {
    let counter = 0;
     
    const factory = (async (_adapterId) => {
      const id = ++counter;
      return {
        host: {
          shutdown: async () => {},
          // First host resolves immediately (simulating already-exited / OOM-killed process)
          // Second host stays pending (still running)
          exited: id === 1 ? Promise.resolve(137) : new Promise<number>(() => {}),
        } as any, // eslint-disable-line @typescript-eslint/no-explicit-any
        conn: {} as any, // eslint-disable-line @typescript-eslint/no-explicit-any
        caps: {} as any, // eslint-disable-line @typescript-eslint/no-explicit-any
        clientDispatcher: { register: () => {}, unregister: () => {} } as any, // eslint-disable-line @typescript-eslint/no-explicit-any
        sessions: new Set<string>(),
        restartPending: false,
        _id: id,
      } as any; // eslint-disable-line @typescript-eslint/no-explicit-any
    }) as PooledEntryFactory;

    const pool = new AcpConnectionPool({ logger: console as any, factory }); // eslint-disable-line @typescript-eslint/no-explicit-any
    const first = await pool.acquire("claude_code");
    // Wait a microtask so the already-resolved Promise settles for the race probe
    await Promise.resolve();
    const second = await pool.acquire("claude_code");
    expect((second as any)._id).not.toBe((first as any)._id); // eslint-disable-line @typescript-eslint/no-explicit-any
  });

  test("shutdownAll clears entries without waiting for idle timer", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({
      logger: console as any, // eslint-disable-line @typescript-eslint/no-explicit-any
      factory,
      idleMs: 10_000,
    });
    const entry = await pool.acquire("claude_code");
    entry.sessions.add("s1");
    await pool.release("claude_code", "s1");
    await pool.shutdownAll();
    // After shutdownAll, next acquire must spawn fresh even with large idleMs
    const again = await pool.acquire("claude_code");
    expect((again as any)._id).not.toBe((entry as any)._id); // eslint-disable-line @typescript-eslint/no-explicit-any
  });
});
