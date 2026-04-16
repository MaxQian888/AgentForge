// T4a placeholder: real implementation lands in T4a (FsSandbox + slice semantics).
// Stub thrown errors are caught by MultiplexedClient tests through their mock ctx.
import type {
  ReadTextFileRequest,
  ReadTextFileResponse,
  WriteTextFileRequest,
  WriteTextFileResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/* eslint-disable @typescript-eslint/no-unused-vars */
export async function readTextFile(
  _ctx: PerSessionContext,
  _p: ReadTextFileRequest,
): Promise<ReadTextFileResponse> {
  throw new Error("fs.readTextFile not yet implemented (T4a)");
}

export async function writeTextFile(
  _ctx: PerSessionContext,
  _p: WriteTextFileRequest,
): Promise<WriteTextFileResponse> {
  throw new Error("fs.writeTextFile not yet implemented (T4a)");
}
