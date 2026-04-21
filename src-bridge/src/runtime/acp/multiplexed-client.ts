import {
  RequestError,
  type Client,
  type CreateElicitationRequest,
  type CreateElicitationResponse,
  type CreateTerminalRequest,
  type CreateTerminalResponse,
  type KillTerminalRequest,
  type KillTerminalResponse,
  type ReadTextFileRequest,
  type ReadTextFileResponse,
  type ReleaseTerminalRequest,
  type ReleaseTerminalResponse,
  type RequestPermissionRequest,
  type RequestPermissionResponse,
  type SessionNotification,
  type TerminalOutputRequest,
  type TerminalOutputResponse,
  type WaitForTerminalExitRequest,
  type WaitForTerminalExitResponse,
  type WriteTextFileRequest,
  type WriteTextFileResponse,
  type ToolCallUpdate,
  type PermissionOption,
  type RequestPermissionOutcome,
} from "@agentclientprotocol/sdk";
import type { Logger } from "./process-host.js";
import * as fsH from "./handlers/fs.js";
import * as termH from "./handlers/terminal.js";
import * as permH from "./handlers/permission.js";
import * as elicH from "./handlers/elicitation.js";
import { mapSessionUpdate } from "./events/session-update.js";

/**
 * Per-task context registered with a `MultiplexedClient` under a
 * specific session id. Inbound `Client` calls carrying that session
 * id are dispatched with this context as the first argument.
 */
export interface PerSessionContext {
  taskId: string;
  cwd: string;
  fsSandbox: {
    resolve(sessionId: string, path: string): string;
  };
  terminalManager: unknown; // concrete type in handlers/terminal.ts (T4b)
  permissionRouter: {
    request(
      taskId: string,
      toolCall: ToolCallUpdate,
      options: PermissionOption[],
    ): Promise<RequestPermissionOutcome>;
  };
  streamer: { emit(event: unknown): void };
  logger: Logger;
}

/**
 * Implements the SDK's `Client` interface for a pooled ACP connection
 * shared across multiple per-task sessions. Dispatches inbound
 * requests/notifications by `params.sessionId` to the registered
 * `PerSessionContext`. Unknown session ids reject with JSON-RPC
 * error code `-32602` ("Invalid params").
 *
 * Notifications (`sessionUpdate`) MUST NOT throw — per ACP contract
 * the client cannot respond with a JSON-RPC error to a one-way
 * message, and the client must continue receiving subsequent
 * updates even after cancellation. Errors inside `sessionUpdate`
 * are logged and swallowed.
 */
export class MultiplexedClient implements Client {
  private sessions = new Map<string, PerSessionContext>();

  constructor(private readonly opts: { logger: Logger }) {}

  register(sessionId: string, ctx: PerSessionContext): void {
    this.sessions.set(sessionId, ctx);
  }

  unregister(sessionId: string): void {
    this.sessions.delete(sessionId);
  }

  has(sessionId: string): boolean {
    return this.sessions.has(sessionId);
  }

  private require(sessionId: string): PerSessionContext {
    const ctx = this.sessions.get(sessionId);
    if (!ctx) {
      throw new RequestError(-32602, "unknown_session", { sessionId });
    }
    return ctx;
  }

  // ── notifications (errors swallowed) ─────────────────────────────
  async sessionUpdate(params: SessionNotification): Promise<void> {
    try {
      const sessionId = (params as unknown as { sessionId: string }).sessionId;
      const ctx = this.sessions.get(sessionId);
      if (!ctx) return;
      const ev = mapSessionUpdate(params);
      ctx.streamer.emit(ev);
    } catch (err) {
      this.opts.logger.warn("sessionUpdate handler failed", err);
    }
  }

  // ── requests (routed by sessionId, delegate to handler modules) ──
  async requestPermission(
    params: RequestPermissionRequest,
  ): Promise<RequestPermissionResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return permH.handle(ctx, params);
  }

  async readTextFile(
    params: ReadTextFileRequest,
  ): Promise<ReadTextFileResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return fsH.readTextFile(ctx, params);
  }

  async writeTextFile(
    params: WriteTextFileRequest,
  ): Promise<WriteTextFileResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return fsH.writeTextFile(ctx, params);
  }

  async createTerminal(
    params: CreateTerminalRequest,
  ): Promise<CreateTerminalResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return termH.createTerminal(ctx, params);
  }

  async terminalOutput(
    params: TerminalOutputRequest,
  ): Promise<TerminalOutputResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return termH.terminalOutput(ctx, params);
  }

  async waitForTerminalExit(
    params: WaitForTerminalExitRequest,
  ): Promise<WaitForTerminalExitResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return termH.waitForExit(ctx, params);
  }

  async killTerminal(
    params: KillTerminalRequest,
  ): Promise<KillTerminalResponse | void> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return termH.kill(ctx, params);
  }

  async releaseTerminal(
    params: ReleaseTerminalRequest,
  ): Promise<ReleaseTerminalResponse | void> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return termH.release(ctx, params);
  }

  async unstable_createElicitation(
    params: CreateElicitationRequest,
  ): Promise<CreateElicitationResponse> {
    const ctx = this.require(
      (params as unknown as { sessionId: string }).sessionId,
    );
    return elicH.createElicitation(ctx, params);
  }
}
