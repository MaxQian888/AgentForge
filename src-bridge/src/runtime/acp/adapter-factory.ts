import { AcpSession } from "./session.js";
import type { AcpConnectionPool } from "./connection-pool.js";
import type { MultiplexedClient, PerSessionContext } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { liveControlsFor, type LiveControlsFlags } from "./capabilities.js";
import { AcpCapabilityUnsupported } from "./errors.js";
import type { Logger } from "./process-host.js";
import type { McpServer } from "@agentclientprotocol/sdk";

export interface AcpTaskInput {
  id: string;
  /** Worktree root (absolute). Becomes the session's cwd + FsSandbox root. */
  worktreeRoot: string;
}

export interface AcpDeps {
  pool: AcpConnectionPool;
  multiplexedClient: MultiplexedClient;
  makeFsSandbox(worktreeRoot: string): PerSessionContext["fsSandbox"];
  terminalManager: unknown;
  permissionRouter: PerSessionContext["permissionRouter"];
  resolveMcpServersFor(task: AcpTaskInput): McpServer[];
  /**
   * Per-adapter thinking-budget advertisement. Not a stable ACP
   * capability yet — adapters that support it (Claude today) pass
   * `true`; others default to `false`. Consumed by liveControlsFor.
   */
  thinkingBudgetAdvertisedFor?(adapterId: AdapterId): boolean;
  logger: Logger;
}

/**
 * The object returned by `createAcpRuntimeAdapter(id)(task, streamer, deps)`.
 * Maps ACP session capabilities onto the existing `RuntimeAdapter`
 * 12-method face consumed by `runtime/registry.ts`. The shape is
 * intentionally loose (each method is optional at compile time) so
 * T6b can plug it into the existing registry without forcing a
 * refactor of `RuntimeAdvancedOperations`.
 */
export interface AcpRuntimeAdapter {
  liveControls: LiveControlsFlags;
  session: AcpSession;
  execute(req: { prompt: string }): Promise<{ stopReason: string }>;
  cancel(): Promise<void>;
  interrupt(): Promise<void>;
  setModel(modelId: string): Promise<void>;
  setMode(modeId: string): Promise<void>;
  setConfigOption(configId: string, value: boolean | string): Promise<unknown>;
  setThinkingBudget(max: number | null): Promise<void>;
  fork(): Promise<AcpSession>;
  executeCommand(command: string): Promise<unknown>;
  dispose(): Promise<void>;
}

export interface AcpRuntimeAdapterFactory {
  (
    task: AcpTaskInput,
    streamer: { emit(event: unknown): void },
    deps: AcpDeps,
  ): Promise<AcpRuntimeAdapter>;
}

/**
 * Build a per-task ACP runtime adapter for a given adapterId.
 *
 * The caller must check the emergency fallback env flag
 * `BRIDGE_ACP_<ADAPTER>=0` BEFORE invoking this factory — the legacy
 * fallback mapping lives in `runtime/registry.ts` (T6b), not here.
 *
 * Unstable methods (`setModel`, `fork`, etc.) throw
 * `AcpCapabilityUnsupported` when the agent has not advertised
 * support. Callers map this to the runtime-capability contract
 * `{support_state:"unsupported", reason_code:<str>}`.
 */
export function createAcpRuntimeAdapter(
  adapterId: AdapterId,
): AcpRuntimeAdapterFactory {
  return async (task, streamer, deps) => {
    const session = await AcpSession.open(deps.pool, {
      taskId: task.id,
      adapterId,
      cwd: task.worktreeRoot,
      streamer,
      permissionRouter: deps.permissionRouter,
      fsSandbox: deps.makeFsSandbox(task.worktreeRoot),
      terminalManager: deps.terminalManager,
      mcpServers: deps.resolveMcpServersFor(task),
      logger: deps.logger,
      multiplexedClient: deps.multiplexedClient,
    });

    const liveControls = liveControlsFor({
      caps: session.capabilities,
      availableModels: session.availableModels,
      availableModes: session.availableModes,
      thinkingBudgetAdvertised:
        deps.thinkingBudgetAdvertisedFor?.(adapterId) ?? false,
    });

    const throwUnsupported = (method: string): never => {
      throw new AcpCapabilityUnsupported(method, "not_advertised");
    };

    return {
      liveControls,
      session,
      async execute(req) {
        const stopReason = await session.prompt([
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          { type: "text", text: req.prompt } as any,
        ]);
        return { stopReason };
      },
      cancel: () => session.cancel(),
      interrupt: () => session.cancel(),
      async setModel(m) {
        if (!liveControls.setModel) throwUnsupported("setModel");
        await session.setModel(m);
      },
      async setMode(m) {
        if (!liveControls.setMode) throwUnsupported("setMode");
        await session.setMode(m);
      },
      async setConfigOption(k, v) {
        return session.setConfigOption(k, v);
      },
      async setThinkingBudget(max) {
        if (!liveControls.setThinkingBudget) throwUnsupported("setThinkingBudget");
        // Adapter-specific wire: Claude's ACP wrapper exposes this via
        // setSessionConfigOption. If the adapter maps it elsewhere,
        // future T10 work updates this line.
        await session.setConfigOption(
          "thinking_budget",
          max == null ? "disabled" : String(max),
        );
      },
      async fork() {
        // Capability gate enforced inside session.forkSession; it throws
        // AcpCapabilityUnsupported if the agent does not advertise it.
        return session.forkSession();
      },
      async executeCommand(command) {
        // Best-effort: prefer an extension RPC; fall back to a slash-prefix
        // prompt. Spec §4.6 documents this fallback explicitly.
        try {
          return await session.extMethod("agent/executeCommand", { command });
        } catch {
          return session.prompt([
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            { type: "text", text: `/run ${command}` } as any,
          ]);
        }
      },
      dispose: () => session.dispose(),
    };
  };
}
