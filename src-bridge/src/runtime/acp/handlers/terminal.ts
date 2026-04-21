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
import type { TerminalManager } from "../terminal-manager.js";

function tm(ctx: PerSessionContext): TerminalManager {
  return ctx.terminalManager as unknown as TerminalManager;
}

export async function createTerminal(
  ctx: PerSessionContext,
  p: CreateTerminalRequest,
): Promise<CreateTerminalResponse> {
  const id = tm(ctx).create({
    command: p.command,
    args: p.args,
    cwd: (p as unknown as { cwd?: string }).cwd ?? ctx.cwd,
    env: (p as unknown as { env?: Record<string, string> }).env,
  });
  return { terminalId: id };
}

export async function terminalOutput(
  ctx: PerSessionContext,
  p: TerminalOutputRequest,
): Promise<TerminalOutputResponse> {
  const o = tm(ctx).getOutput(p.terminalId);
  return {
    output: o.output,
    truncated: o.truncated,
    exitStatus: o.exitStatus
      ? { exitCode: o.exitStatus.exitCode, signal: o.exitStatus.signal }
      : null,
  };
}

export async function waitForExit(
  ctx: PerSessionContext,
  p: WaitForTerminalExitRequest,
): Promise<WaitForTerminalExitResponse> {
  const x = await tm(ctx).waitForExit(p.terminalId);
  return { exitCode: x.exitCode, signal: x.signal };
}

export async function kill(
  ctx: PerSessionContext,
  p: KillTerminalRequest,
): Promise<KillTerminalResponse | void> {
  tm(ctx).kill(p.terminalId);
}

export async function release(
  ctx: PerSessionContext,
  p: ReleaseTerminalRequest,
): Promise<ReleaseTerminalResponse | void> {
  tm(ctx).release(p.terminalId);
}
