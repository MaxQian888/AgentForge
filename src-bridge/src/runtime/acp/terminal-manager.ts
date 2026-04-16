import { RequestError } from "@agentclientprotocol/sdk";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";

export interface TerminalManagerOpts {
  /** Per-terminal byte cap for the output ring buffer (default 10 MB). */
  perTaskByteLimit?: number;
  /** Global cap on concurrently-running terminals (default 16). */
  maxConcurrent?: number;
}

export interface TerminalExitInfo {
  exitCode: number | null;
  signal: string | null;
}

interface TerminalEntry {
  proc: ChildProcessWithoutNullStreams;
  buffer: string[];
  bytes: number;
  truncated: boolean;
  exited: Promise<TerminalExitInfo>;
}

/**
 * Per-bridge terminal pool. Each entry is a child process with its own
 * output ring buffer. Over-capacity allocation rejects with JSON-RPC
 * `-32000 terminal_capacity` so the agent receives a structured error.
 *
 * Output buffering is bounded: when the running total exceeds
 * `perTaskByteLimit`, oldest chunks are dropped and `truncated` is
 * flagged. Callers read via `getOutput(id)`, which also reports
 * `exitStatus` when the child has exited.
 */
export class TerminalManager {
  private terminals = new Map<string, TerminalEntry>();
  private readonly byteLimit: number;
  private readonly maxConcurrent: number;

  constructor(opts: TerminalManagerOpts = {}) {
    this.byteLimit = opts.perTaskByteLimit ?? 10 * 1024 * 1024;
    this.maxConcurrent = opts.maxConcurrent ?? 16;
  }

  create(spec: {
    command: string;
    args?: readonly string[];
    cwd?: string;
    env?: Record<string, string>;
  }): string {
    if (this.terminals.size >= this.maxConcurrent) {
      throw new RequestError(-32000, "terminal_capacity", {
        reason: "terminal_capacity",
        max: this.maxConcurrent,
      });
    }
    const proc = spawn(spec.command, [...(spec.args ?? [])], {
      cwd: spec.cwd,
      env: { ...process.env, ...(spec.env ?? {}) },
      stdio: ["pipe", "pipe", "pipe"],
    });
    const entry: TerminalEntry = {
      proc,
      buffer: [],
      bytes: 0,
      truncated: false,
      exited: new Promise<TerminalExitInfo>((r) =>
        proc.on("exit", (code, sig) => r({ exitCode: code, signal: sig })),
      ),
    };
    const sink = (chunk: Buffer) => {
      const s = chunk.toString("utf8");
      entry.buffer.push(s);
      entry.bytes += s.length;
      while (entry.bytes > this.byteLimit && entry.buffer.length > 1) {
        entry.bytes -= entry.buffer.shift()!.length;
        entry.truncated = true;
      }
    };
    proc.stdout.on("data", sink);
    proc.stderr.on("data", sink);
    const id = randomUUID();
    this.terminals.set(id, entry);
    return id;
  }

  getOutput(id: string): {
    output: string;
    truncated: boolean;
    exitStatus?: TerminalExitInfo;
  } {
    const e = this.mustGet(id);
    const exitStatus =
      e.proc.exitCode != null || e.proc.signalCode != null
        ? { exitCode: e.proc.exitCode, signal: e.proc.signalCode ?? null }
        : undefined;
    return { output: e.buffer.join(""), truncated: e.truncated, exitStatus };
  }

  async waitForExit(id: string): Promise<TerminalExitInfo> {
    const e = this.mustGet(id);
    return e.exited;
  }

  kill(id: string): void {
    const e = this.mustGet(id);
    if (e.proc.exitCode == null) e.proc.kill("SIGKILL");
  }

  release(id: string): void {
    const e = this.terminals.get(id);
    if (!e) return;
    if (e.proc.exitCode == null) e.proc.kill("SIGKILL");
    this.terminals.delete(id);
  }

  /** Test helper — total active terminal count. */
  size(): number {
    return this.terminals.size;
  }

  private mustGet(id: string): TerminalEntry {
    const e = this.terminals.get(id);
    if (!e) {
      throw new RequestError(-32602, "unknown_terminal", {
        reason: "unknown_terminal",
        terminalId: id,
      });
    }
    return e;
  }
}
