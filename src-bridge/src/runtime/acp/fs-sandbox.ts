import { realpathSync, existsSync } from "node:fs";
import { isAbsolute, resolve as resolvePath, dirname } from "node:path";
import { RequestError } from "@agentclientprotocol/sdk";

/**
 * Resolves agent-supplied paths against a worktree root and rejects any
 * path that escapes the root after `realpath()` (follows symlinks).
 *
 * Non-compliant calls reject with JSON-RPC error `-32602` / `"Invalid
 * params"` and `data.reason = "path_escapes_worktree"` per spec §6.1.
 * We do NOT fall back to the outer filesystem.
 *
 * The `sessionId` parameter is accepted for future per-session policy
 * hooks but is not currently consulted; all sessions sharing the same
 * `FsSandbox` instance see the same root.
 */
export class FsSandbox {
  private readonly rootReal: string;

  constructor(private readonly worktreeRoot: string) {
    this.rootReal = realpathSync(worktreeRoot);
  }

  resolve(_sessionId: string, requested: string): string {
    const abs = isAbsolute(requested)
      ? requested
      : resolvePath(this.worktreeRoot, requested);
    // realpath on the deepest existing ancestor so symlinks are followed.
    // For non-existent targets (e.g., writing a new file) we resolve up to
    // the parent and assume the non-existent tail stays within the root.
    let check = abs;
    while (!existsSync(check) && dirname(check) !== check) {
      check = dirname(check);
    }
    const real = existsSync(check) ? realpathSync(check) : check;
    if (!real.startsWith(this.rootReal)) {
      throw new RequestError(-32602, "path_escapes_worktree", {
        reason: "path_escapes_worktree",
        requested,
        resolved: real,
      });
    }
    return abs;
  }
}
