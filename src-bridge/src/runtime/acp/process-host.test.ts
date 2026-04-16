import { describe, expect, test } from "bun:test";
import { ChildProcessHost } from "./process-host.js";

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
    expect(await host.exited).toBeGreaterThanOrEqual(0);
  });
});
