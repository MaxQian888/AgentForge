import { describe, expect, test } from "bun:test";
import { readTextFile, writeTextFile } from "./fs.js";
import { FsSandbox } from "../fs-sandbox.js";
import { mkdtempSync, writeFileSync, readFileSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import type { PerSessionContext } from "../multiplexed-client.js";

function mkCtx(root: string): PerSessionContext {
  return {
    taskId: "t1",
    cwd: root,
    fsSandbox: new FsSandbox(root),
    terminalManager: {},
    permissionRouter: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      request: async () => ({ outcome: "selected", optionId: "allow" } as any),
    },
    streamer: { emit: () => {} },
    logger: { debug: () => {}, warn: () => {}, error: () => {} },
  };
}

describe("fs handler", () => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "fs-h-")));
  writeFileSync(join(root, "a.txt"), "line1\nline2\nline3\nline4\n");

  test("readTextFile returns entire UTF-8 content", async () => {
    const ctx = mkCtx(root);
    const res = await readTextFile(ctx, {
      sessionId: "s1",
      path: "a.txt",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.content).toBe("line1\nline2\nline3\nline4\n");
  });

  test("readTextFile honors 1-based line+limit slice", async () => {
    const ctx = mkCtx(root);
    const res = await readTextFile(ctx, {
      sessionId: "s1",
      path: "a.txt",
      line: 2,
      limit: 2,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.content).toBe("line2\nline3\n");
  });

  test("readTextFile with line only reads to EOF", async () => {
    const ctx = mkCtx(root);
    const res = await readTextFile(ctx, {
      sessionId: "s1",
      path: "a.txt",
      line: 3,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.content).toBe("line3\nline4\n");
  });

  test("writeTextFile creates new file inside worktree", async () => {
    const ctx = mkCtx(root);
    await writeTextFile(ctx, {
      sessionId: "s1",
      path: "b.txt",
      content: "hello",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(readFileSync(join(root, "b.txt"), "utf8")).toBe("hello");
  });

  test("writeTextFile overwrites existing file", async () => {
    writeFileSync(join(root, "overwrite.txt"), "old");
    const ctx = mkCtx(root);
    await writeTextFile(ctx, {
      sessionId: "s1",
      path: "overwrite.txt",
      content: "new",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(readFileSync(join(root, "overwrite.txt"), "utf8")).toBe("new");
  });

  test("path escape rejected through handler", async () => {
    const ctx = mkCtx(root);
    await expect(
      readTextFile(ctx, {
        sessionId: "s1",
        path: "../outside.txt",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any),
    ).rejects.toMatchObject({ code: -32602 });
  });
});
