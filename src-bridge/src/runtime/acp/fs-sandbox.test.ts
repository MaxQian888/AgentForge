import { describe, expect, test } from "bun:test";
import { FsSandbox } from "./fs-sandbox.js";
import { mkdtempSync, symlinkSync, writeFileSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

describe("FsSandbox", () => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "sandbox-")));
  const outsideRoot = realpathSync(mkdtempSync(join(tmpdir(), "outside-")));
  writeFileSync(join(root, "in.txt"), "content");
  writeFileSync(join(outsideRoot, "secret.txt"), "secret");

  test("resolves file inside worktree", () => {
    const sb = new FsSandbox(root);
    expect(sb.resolve("s1", "in.txt")).toBe(join(root, "in.txt"));
  });

  test("rejects absolute path outside worktree", () => {
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", join(outsideRoot, "secret.txt"))).toThrow(
      /path_escapes_worktree/,
    );
  });

  test("rejects ../ traversal", () => {
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", "../secret.txt")).toThrow(
      /path_escapes_worktree/,
    );
  });

  test("rejects symlink that resolves outside worktree", () => {
    symlinkSync(outsideRoot, join(root, "link-out"), "dir");
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", "link-out/secret.txt")).toThrow(
      /path_escapes_worktree/,
    );
  });

  test("allows non-existent file inside worktree (write path)", () => {
    const sb = new FsSandbox(root);
    const resolved = sb.resolve("s1", "new-file.txt");
    expect(resolved).toBe(join(root, "new-file.txt"));
  });

  test("RequestError carries JSON-RPC code and data", () => {
    const sb = new FsSandbox(root);
    try {
      sb.resolve("s1", "../secret.txt");
      throw new Error("should have thrown");
    } catch (err) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const e = err as any;
      expect(e.code).toBe(-32602);
      expect(e.data?.reason).toBe("path_escapes_worktree");
    }
  });
});
