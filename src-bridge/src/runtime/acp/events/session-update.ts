// T5 placeholder: full mapping table lands in T5 (spec §8).
// This stub emits a passthrough shape so MultiplexedClient tests can assert
// emit() was called with a session-scoped payload.
import type { SessionNotification } from "@agentclientprotocol/sdk";

export interface MappedEvent {
  type: string;
  session_id: string;
  metadata?: { _meta?: unknown; _raw?: unknown };
  [k: string]: unknown;
}

export function mapSessionUpdate(n: SessionNotification): MappedEvent {
  return {
    type: "status_change",
    session_id: (n as unknown as { sessionId: string }).sessionId,
    kind: "acp_passthrough",
    metadata: { _raw: (n as unknown as { update: unknown }).update },
  };
}
