import type { SessionNotification } from "@agentclientprotocol/sdk";

/**
 * Shape of the event fanned out to the per-task `EventStreamer`. The
 * existing bridge `AgentEvent` discriminator lives in
 * `src-bridge/src/types.ts`; this module emits plain records whose
 * `type` field matches one of those discriminator values. Additional
 * ACP-specific fields ride along and downstream consumers access them
 * opportunistically.
 */
export interface MappedEvent {
  type: string;
  session_id: string;
  metadata?: { _meta?: unknown; _raw?: unknown };
  [k: string]: unknown;
}

/**
 * Single source of truth for the ACP → AgentEventType mapping (spec §8).
 *
 * Unknown SessionUpdate variants fall through to
 * `status_change{kind:"acp_passthrough"}` with the original payload
 * stored at `metadata._raw`, so schema extensions don't silently drop
 * data before downstream consumers opt in.
 *
 * The `_meta` field on the notification is copied verbatim to
 * `AgentEvent.metadata._meta`, allowing adapter-specific usage /
 * telemetry to surface without a bridge-level schema bump.
 */
 
export function mapSessionUpdate(n: SessionNotification): MappedEvent {
  const sessionId = (n as unknown as { sessionId: string }).sessionId;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const upd = (n as unknown as { update: any }).update;
  const notifMeta = (n as unknown as { _meta?: unknown })._meta;
  const updMeta = upd?._meta;
  // Prefer notification-level _meta, fall back to update-level.
  const meta = notifMeta ?? updMeta;
  const metaBag = meta !== undefined ? { _meta: meta } : {};

  switch (upd?.sessionUpdate) {
    case "user_message_chunk":
      return {
        type: "partial_message",
        session_id: sessionId,
        direction: "user",
        content: upd.content,
        metadata: metaBag,
      };

    case "agent_message_chunk":
      return {
        type: "output",
        session_id: sessionId,
        content: upd.content,
        text: extractText(upd.content),
        metadata: metaBag,
      };

    case "agent_thought_chunk":
      return {
        type: "reasoning",
        session_id: sessionId,
        content: upd.content,
        text: extractText(upd.content),
        metadata: metaBag,
      };

    case "tool_call":
      return {
        type: "tool_call",
        session_id: sessionId,
        tool_call_id: upd.toolCallId,
        title: upd.title,
        kind: upd.kind,
        status: upd.status,
        content: upd.content,
        rawInput: upd.rawInput,
        locations: upd.locations,
        metadata: metaBag,
      };

    case "tool_call_update": {
      const terminal = upd.status === "completed" || upd.status === "failed";
      return {
        type: terminal ? "tool_result" : "tool.status_change",
        session_id: sessionId,
        tool_call_id: upd.toolCallId,
        status: upd.status,
        title: upd.title,
        content: upd.content,
        rawOutput: upd.rawOutput,
        locations: upd.locations,
        metadata: metaBag,
      };
    }

    case "plan":
      return {
        type: "todo_update",
        session_id: sessionId,
        entries: upd.entries,
        metadata: metaBag,
      };

    case "available_commands_update":
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "commands",
        commands: upd.availableCommands ?? upd.commands,
        metadata: metaBag,
      };

    case "current_mode_update":
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "mode",
        mode_id: upd.currentModeId,
        metadata: metaBag,
      };

    case "config_option_update":
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "config_option",
        option_id: upd.configId ?? upd.optionId,
        value: upd.value,
        metadata: metaBag,
      };

    case "session_info_update":
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "session_info",
        info: upd,
        metadata: metaBag,
      };

    case "usage_update":
      return {
        type: "cost_update",
        session_id: sessionId,
        usage: upd.usage ?? upd,
        metadata: metaBag,
      };

    default:
      // Unknown variants (future schema additions, unstable events not
      // yet promoted, or extension-shaped payloads) fall through to a
      // structured passthrough so nothing is silently dropped.
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "acp_passthrough",
        metadata: { ...metaBag, _raw: upd },
      };
  }
}

/**
 * Best-effort text extraction from a ContentBlock. Returns empty
 * string for non-text variants (image/audio/resource/...) so
 * downstream consumers expecting `text` don't crash; the full
 * content lives under `content`.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function extractText(content: any): string {
  if (!content) return "";
  if (typeof content === "string") return content;
  if (content.type === "text" && typeof content.text === "string") return content.text;
  return "";
}
