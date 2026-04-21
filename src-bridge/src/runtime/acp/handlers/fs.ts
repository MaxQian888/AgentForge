import { readFile, writeFile } from "node:fs/promises";
import type {
  ReadTextFileRequest,
  ReadTextFileResponse,
  WriteTextFileRequest,
  WriteTextFileResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/**
 * fs/readTextFile: reads a UTF-8 text file from the session's worktree.
 * Honors optional `line` (1-based start line) + `limit` (number of
 * lines) per ACP schema. Path escape attempts reject via FsSandbox
 * with `-32602 path_escapes_worktree`.
 */
export async function readTextFile(
  ctx: PerSessionContext,
  params: ReadTextFileRequest,
): Promise<ReadTextFileResponse> {
  const sessionId = (params as unknown as { sessionId: string }).sessionId;
  const abs = ctx.fsSandbox.resolve(sessionId, params.path);
  const full = await readFile(abs, "utf8");

  const line = (params as unknown as { line?: number }).line;
  const limit = (params as unknown as { limit?: number }).limit;
  if (line == null && limit == null) {
    return { content: full };
  }
  const lines = full.split("\n");
  const start = Math.max(0, (line ?? 1) - 1);
  const end = limit != null ? Math.min(lines.length, start + limit) : lines.length;
  const slice = lines.slice(start, end).join("\n");
  // Preserve trailing newline when the slice does not reach EOF and the
  // original had one at the last emitted line.
  const trailing = end < lines.length ? "\n" : "";
  return { content: slice + trailing };
}

/**
 * fs/writeTextFile: writes UTF-8 content to a file inside the session
 * worktree. Creates file if missing; overwrites if present. Binary
 * writes are out of scope this phase.
 */
export async function writeTextFile(
  ctx: PerSessionContext,
  params: WriteTextFileRequest,
): Promise<WriteTextFileResponse> {
  const sessionId = (params as unknown as { sessionId: string }).sessionId;
  const abs = ctx.fsSandbox.resolve(sessionId, params.path);
  await writeFile(abs, params.content, "utf8");
  return {} as WriteTextFileResponse;
}
