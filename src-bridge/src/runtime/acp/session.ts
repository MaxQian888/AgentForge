import type {
  AgentCapabilities,
  ClientSideConnection,
  ContentBlock,
  McpServer,
  ModelInfo,
  SessionMode,
  SetSessionConfigOptionResponse,
  StopReason,
} from "@agentclientprotocol/sdk";
import type { AcpConnectionPool, PooledEntry } from "./connection-pool.js";
import type { MultiplexedClient, PerSessionContext } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { gateUnstable } from "./capabilities.js";
import { AcpConcurrentPrompt } from "./errors.js";
import type { Logger } from "./process-host.js";

export interface AcpSessionOptions {
  taskId: string;
  adapterId: AdapterId;
  cwd: string;
  streamer: { emit(event: unknown): void };
  permissionRouter: PerSessionContext["permissionRouter"];
  fsSandbox: PerSessionContext["fsSandbox"];
  terminalManager: unknown;
  mcpServers: McpServer[];
  logger: Logger;
  multiplexedClient: MultiplexedClient;
}

/**
 * Per-(task, session) wrapper over a pooled ACP connection. What
 * `runtime/registry.ts`'s adapter factory holds and maps to the
 * `RuntimeAdapter` 12-method face. Stable methods delegate directly
 * to the SDK connection; unstable methods are capability-gated and
 * throw `AcpCapabilityUnsupported` when the agent has not advertised
 * support.
 *
 * Invariants:
 * - One `AcpSession` wraps exactly one `(pooledEntry, sessionId)` pair.
 * - `prompt` serializes per-session; concurrent call rejects
 *   immediately with `AcpConcurrentPrompt`.
 * - All capability checks read `this.capabilities` (cached on
 *   `AcpSession.open`).
 */
export class AcpSession {
  private promptInFlight = false;

  /** Per-session mode state (if agent supports session modes). */
  readonly availableModes: readonly SessionMode[];
  /** Per-session model state (unstable; if agent advertises). */
  readonly availableModels: readonly ModelInfo[];

  static async open(
    pool: AcpConnectionPool,
    opts: AcpSessionOptions,
  ): Promise<AcpSession> {
    const entry = await pool.acquire(opts.adapterId);
    const perSessionCtx: PerSessionContext = {
      taskId: opts.taskId,
      cwd: opts.cwd,
      fsSandbox: opts.fsSandbox,
      terminalManager: opts.terminalManager,
      permissionRouter: opts.permissionRouter,
      streamer: opts.streamer,
      logger: opts.logger,
    };
    const res = await entry.conn.newSession({
      cwd: opts.cwd,
      mcpServers: opts.mcpServers,
    });
    entry.sessions.add(res.sessionId);
    opts.multiplexedClient.register(res.sessionId, perSessionCtx);
    return new AcpSession(
      pool,
      entry,
      res.sessionId,
      res.modes?.availableModes ?? [],
      res.models?.availableModels ?? [],
      opts,
    );
  }

  private constructor(
    private readonly pool: AcpConnectionPool,
    private readonly entry: PooledEntry,
    readonly sessionId: string,
    availableModes: readonly SessionMode[],
    availableModels: readonly ModelInfo[],
    private readonly opts: AcpSessionOptions,
  ) {
    this.availableModes = availableModes;
    this.availableModels = availableModels;
  }

  get capabilities(): AgentCapabilities {
    return this.entry.caps;
  }

  private get conn(): ClientSideConnection {
    return this.entry.conn;
  }

  // ── stable methods ────────────────────────────────────────────────
  async prompt(content: ContentBlock[]): Promise<StopReason> {
    if (this.promptInFlight) {
      throw new AcpConcurrentPrompt(
        `prompt already in flight for session ${this.sessionId}`,
      );
    }
    this.promptInFlight = true;
    try {
      const res = await this.conn.prompt({
        sessionId: this.sessionId,
        prompt: content,
      });
      return res.stopReason;
    } finally {
      this.promptInFlight = false;
    }
  }

  async cancel(): Promise<void> {
    await this.conn.cancel({ sessionId: this.sessionId });
  }

  async setMode(modeId: string): Promise<void> {
    await this.conn.setSessionMode({ sessionId: this.sessionId, modeId });
  }

  async setConfigOption(
    configId: string,
    value: boolean | string,
  ): Promise<SetSessionConfigOptionResponse> {
    // SDK discriminates the request on value type. Boolean branch requires
    // `type: "boolean"`; string branch takes the value as a SessionConfigValueId.
    if (typeof value === "boolean") {
      return this.conn.setSessionConfigOption({
        sessionId: this.sessionId,
        configId,
        type: "boolean",
        value,
      });
    }
    return this.conn.setSessionConfigOption({
      sessionId: this.sessionId,
      configId,
      value,
    });
  }

  async authenticate(methodId: string): Promise<void> {
    await this.conn.authenticate({ methodId });
  }

  // ── unstable methods (capability-gated) ──────────────────────────
  async forkSession(): Promise<AcpSession> {
    gateUnstable(this.capabilities, "forkSession", "sessionCapabilities.fork");
    const res = await this.conn.unstable_forkSession({
      sessionId: this.sessionId,
      cwd: this.opts.cwd,
      mcpServers: this.opts.mcpServers,
    });
    this.entry.sessions.add(res.sessionId);
    this.opts.multiplexedClient.register(res.sessionId, {
      taskId: this.opts.taskId,
      cwd: this.opts.cwd,
      fsSandbox: this.opts.fsSandbox,
      terminalManager: this.opts.terminalManager,
      permissionRouter: this.opts.permissionRouter,
      streamer: this.opts.streamer,
      logger: this.opts.logger,
    });
    return new AcpSession(
      this.pool,
      this.entry,
      res.sessionId,
      res.modes?.availableModes ?? [],
      res.models?.availableModels ?? [],
      this.opts,
    );
  }

  async resumeSession(): Promise<void> {
    gateUnstable(
      this.capabilities,
      "resumeSession",
      "sessionCapabilities.resume",
    );
    await this.conn.unstable_resumeSession({
      sessionId: this.sessionId,
      cwd: this.opts.cwd,
      mcpServers: this.opts.mcpServers,
    });
  }

  async setModel(modelId: string): Promise<void> {
    gateUnstable(
      this.capabilities,
      "setModel",
      "sessionCapabilities.setModel",
    );
    await this.conn.unstable_setSessionModel({
      sessionId: this.sessionId,
      modelId,
    });
  }

  async closeSession(): Promise<void> {
    gateUnstable(
      this.capabilities,
      "closeSession",
      "sessionCapabilities.close",
    );
    await this.conn.unstable_closeSession({ sessionId: this.sessionId });
  }

  async logout(): Promise<void> {
    gateUnstable(
      this.capabilities,
      "logout",
      "authenticationCapabilities.logout",
    );
    await this.conn.unstable_logout({});
  }

  async extMethod(
    method: string,
    params: Record<string, unknown>,
  ): Promise<Record<string, unknown>> {
    return this.conn.extMethod(method, {
      sessionId: this.sessionId,
      ...params,
    });
  }

  async extNotification(
    method: string,
    params: Record<string, unknown>,
  ): Promise<void> {
    await this.conn.extNotification(method, {
      sessionId: this.sessionId,
      ...params,
    });
  }

  // ── lifecycle ────────────────────────────────────────────────────
  async dispose(): Promise<void> {
    this.opts.multiplexedClient.unregister(this.sessionId);
    await this.pool.release(this.opts.adapterId, this.sessionId);
  }
}
