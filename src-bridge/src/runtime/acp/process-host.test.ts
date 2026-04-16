import { describe, expect, test } from "bun:test";
import { ChildProcessHost, RingBuffer } from "./process-host.js";

describe("ChildProcessHost", () => {
  test("spawns child and exposes stdin/stdout streams", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "process.stdin.pipe(process.stdout)"],
      env: {},
      logger: console,
    });
    const io = await host.start();
    expect(io.stdin).toBeDefined();
    expect(io.stdout).toBeDefined();
    await host.shutdown(100);
  });

  test("captures stderr into ring buffer", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "console.error('boom'); setInterval(()=>{},1000)"],
      env: {},
      logger: console,
    });
    await host.start();
    await new Promise((r) => setTimeout(r, 200));
    expect(host.stderrBuffer.tail()).toContain("boom");
    await host.shutdown(100);
  });

  test("throws AcpCommandNotFound when binary missing", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "definitely-not-a-real-binary-xyz",
      args: [],
      env: {},
      logger: console,
    });
    await expect(host.start()).rejects.toMatchObject({
      name: "AcpCommandNotFound",
    });
  });

  test("shutdown: closes stdin then SIGTERM then SIGKILL", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
      env: {},
      logger: console,
    });
    await host.start();
    const t0 = Date.now();
    await host.shutdown(50);
    expect(Date.now() - t0).toBeLessThan(3000);
    const code = await host.exited;
    expect(code === null || code >= 0).toBe(true);
  });

  test("start() called twice throws", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
      env: {},
      logger: console,
    });
    await host.start();
    await expect(host.start()).rejects.toThrow(/called twice/);
    await host.shutdown(100);
  });
});

describe("RingBuffer", () => {
  test("keeps trailing bytes when one chunk exceeds limit", () => {
    const rb = new RingBuffer(100);
    rb.append("A".repeat(250));
    const tail = rb.tail();
    expect(tail.length).toBe(100);
    expect(tail).toBe("A".repeat(100));
  });

  test("evicts oldest parts when sum exceeds limit", () => {
    const rb = new RingBuffer(10);
    rb.append("aaaa");  // 4
    rb.append("bbbb");  // 8
    rb.append("cccc");  // 12 → evict "aaaa" → 8
    rb.append("dddd");  // 12 → evict "bbbb" → 8
    expect(rb.tail()).toBe("ccccdddd");
  });

  test("retains below-limit contents as-is", () => {
    const rb = new RingBuffer(100);
    rb.append("hello ");
    rb.append("world");
    expect(rb.tail()).toBe("hello world");
  });
});
