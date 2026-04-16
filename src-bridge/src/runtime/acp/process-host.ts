import { AcpCommandNotFound } from "./errors.js";
import type { AdapterId } from "./registry.js";

export interface Logger {
  debug(msg: string, meta?: unknown): void;
  warn(msg: string, meta?: unknown): void;
  error(msg: string, meta?: unknown): void;
}

export interface ChildProcessHostOptions {
  adapterId: AdapterId;
  command: string;
  args: readonly string[];
  env: Record<string, string>;
  logger: Logger;
}

export class RingBuffer {
  private parts: string[] = [];
  private bytes = 0;
  constructor(private readonly limit: number) {}
  append(chunk: string): void {
    if (chunk.length >= this.limit) {
      this.parts = [chunk.slice(-this.limit)];
      this.bytes = this.parts[0].length;
      return;
    }
    this.parts.push(chunk);
    this.bytes += chunk.length;
    while (this.bytes > this.limit && this.parts.length > 1) {
      this.bytes -= this.parts.shift()!.length;
    }
  }
  tail(): string {
    return this.parts.join("");
  }
}

export class ChildProcessHost {
  readonly stderrBuffer = new RingBuffer(8 * 1024);
  private proc?: ReturnType<typeof Bun.spawn>;
  private resolveExit!: (code: number | null) => void;
  readonly exited = new Promise<number | null>((r) => {
    this.resolveExit = r;
  });
  private startingEnv: Record<string, string>;

  constructor(private readonly opts: ChildProcessHostOptions) {
    this.startingEnv = { ...process.env, ...opts.env } as Record<string, string>;
  }

  async start(): Promise<{
    stdin: WritableStream<Uint8Array>;
    stdout: ReadableStream<Uint8Array>;
  }> {
    if (this.proc) {
      throw new Error("ChildProcessHost.start() called twice");
    }
    try {
      this.proc = Bun.spawn([this.opts.command, ...this.opts.args], {
        stdin: "pipe",
        stdout: "pipe",
        stderr: "pipe",
        env: this.startingEnv,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (/ENOENT|not found|not recognized/i.test(msg)) {
        throw new AcpCommandNotFound(this.opts.adapterId, this.opts.command);
      }
      throw err;
    }

    // Drain stderr into ring buffer
    (async () => {
      const reader = (this.proc!.stderr as ReadableStream<Uint8Array>).getReader();
      const dec = new TextDecoder();
      for (;;) {
        const { value, done } = await reader.read();
        if (done) return;
        this.stderrBuffer.append(dec.decode(value));
      }
    })().catch((e) => this.opts.logger.warn("stderr drain error", e));

    this.proc.exited.then((code) => this.resolveExit(code ?? null));

    return {
      stdin: this.proc.stdin as unknown as WritableStream<Uint8Array>,
      stdout: this.proc.stdout as unknown as ReadableStream<Uint8Array>,
    };
  }

  async shutdown(gracefulMs = 2000): Promise<void> {
    if (!this.proc) return;
    try {
      // Close stdin (signals EOF to agents that watch it)
      try {
        // stdin is FileSink when spawned with stdin:"pipe"; end() flushes + closes
        (this.proc.stdin as import("bun").FileSink).end();
      } catch {
        /* already closed */
      }
      const raced = await Promise.race([
        this.exited,
        new Promise<null>((r) => setTimeout(() => r(null), gracefulMs)),
      ]);
      if (raced === null) {
        this.proc.kill("SIGTERM");
        const killed = await Promise.race([
          this.exited,
          new Promise<null>((r) => setTimeout(() => r(null), 3000)),
        ]);
        if (killed === null) this.proc.kill("SIGKILL");
      }
    } finally {
      try {
        await this.exited;
      } catch {
        /* swallowed */
      }
    }
  }
}
