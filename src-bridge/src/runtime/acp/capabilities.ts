import type {
  AgentCapabilities,
  ModelInfo,
  SessionMode,
} from "@agentclientprotocol/sdk";
import { AcpCapabilityUnsupported } from "./errors.js";

/**
 * Throws `AcpCapabilityUnsupported` when the capability identified by a
 * dotted path is not advertised (absent or falsy) in the negotiated
 * `AgentCapabilities`. `method` is the public face the caller invoked
 * (e.g., `"forkSession"`); `path` is the schema location checked
 * (e.g., `"sessionCapabilities.fork"`). The thrown error's `reason` is
 * `no_capability_<path>`.
 */
export function gateUnstable(
  caps: AgentCapabilities,
  method: string,
  path: string,
): void {
  const segments = path.split(".");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let cur: any = caps;
  for (const s of segments) {
    if (cur == null || typeof cur !== "object" || !(s in cur)) {
      throw new AcpCapabilityUnsupported(method, `no_capability_${path}`);
    }
    cur = cur[s];
  }
  if (!cur) throw new AcpCapabilityUnsupported(method, `no_capability_${path}`);
}

export interface LiveControlsFlags {
  setModel: boolean;
  setMode: boolean;
  setThinkingBudget: boolean;
  setConfigOption: boolean;
  mcpServerStatus: boolean;
}

/**
 * Session-scoped inputs that `liveControlsFor` needs alongside
 * `AgentCapabilities`. `availableModes` ships in `NewSessionResponse`;
 * `availableModels` ships in the unstable `SessionModelState` side
 * channel. `thinkingBudgetAdvertised` is a per-adapter signal because
 * the current ACP schema does not expose it in `PromptCapabilities` —
 * adapters that support it (e.g., Claude) pass `true` explicitly.
 */
export interface LiveControlsInput {
  caps: AgentCapabilities;
  availableModels?: readonly ModelInfo[];
  availableModes?: readonly SessionMode[];
  thinkingBudgetAdvertised?: boolean;
}

/**
 * Derives the `live_controls` flag set from an agent's advertised
 * capabilities + per-session state. Consumed by `adapter-factory.ts`
 * to drop the legacy `runtime === "claude_code"` gate (spec §7.3)
 * and by `agent-runtime.ts` when populating the WS `live_controls`
 * payload.
 */
export function liveControlsFor(info: LiveControlsInput): LiveControlsFlags {
  const hasModels = (info.availableModels?.length ?? 0) > 0;
  const hasModes = (info.availableModes?.length ?? 0) > 0;
  const mcp = info.caps.mcpCapabilities ?? {};
  return {
    setModel: hasModels,
    setMode: hasModes,
    setThinkingBudget: info.thinkingBudgetAdvertised === true,
    // `session/setSessionConfigOption` is stable; always available once a session exists.
    setConfigOption: true,
    mcpServerStatus: Boolean(mcp.http || mcp.sse),
  };
}
