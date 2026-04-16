// T4b placeholder: real implementation lands in T4b (TerminalManager PTY pool).
import type {
  CreateTerminalRequest,
  CreateTerminalResponse,
  KillTerminalRequest,
  KillTerminalResponse,
  ReleaseTerminalRequest,
  ReleaseTerminalResponse,
  TerminalOutputRequest,
  TerminalOutputResponse,
  WaitForTerminalExitRequest,
  WaitForTerminalExitResponse,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

/* eslint-disable @typescript-eslint/no-unused-vars */
export async function createTerminal(
  _c: PerSessionContext,
  _p: CreateTerminalRequest,
): Promise<CreateTerminalResponse> {
  throw new Error("terminal.create not yet implemented (T4b)");
}

export async function terminalOutput(
  _c: PerSessionContext,
  _p: TerminalOutputRequest,
): Promise<TerminalOutputResponse> {
  throw new Error("terminal.output not yet implemented (T4b)");
}

export async function waitForExit(
  _c: PerSessionContext,
  _p: WaitForTerminalExitRequest,
): Promise<WaitForTerminalExitResponse> {
  throw new Error("terminal.waitForExit not yet implemented (T4b)");
}

export async function kill(
  _c: PerSessionContext,
  _p: KillTerminalRequest,
): Promise<KillTerminalResponse | void> {
  throw new Error("terminal.kill not yet implemented (T4b)");
}

export async function release(
  _c: PerSessionContext,
  _p: ReleaseTerminalRequest,
): Promise<ReleaseTerminalResponse | void> {
  throw new Error("terminal.release not yet implemented (T4b)");
}
