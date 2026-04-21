#!/usr/bin/env bun
/*
 * Minimal ACP agent fixture for component tests. Speaks stable JSON-RPC over
 * stdio via the SDK's AgentSideConnection. Handles:
 *   - initialize → declares minimal agent capabilities
 *   - newSession → returns incrementing sessionId, tracks per-session state
 *   - prompt → emits 5 agent_message_chunk ticks (50ms apart) then end_turn.
 *              If first text block is "FS_READ", instead calls back into the
 *              client's readTextFile and emits the content. If "CANCEL_ME",
 *              loops indefinitely until cancel notification flips state.
 *   - cancel → flips per-session cancelled flag; prompt loop observes and
 *              returns stopReason="cancelled".
 *   - setSessionMode / setSessionConfigOption / authenticate → no-op
 *
 * Not exhaustive — only enough for the component tests to verify wiring.
 */
import {
  AgentSideConnection,
  ndJsonStream,
  type Agent,
  type InitializeRequest,
  type NewSessionRequest,
  type PromptRequest,
  type CancelNotification,
  type SetSessionModeRequest,
  type SetSessionConfigOptionRequest,
  type AuthenticateRequest,
  type LoadSessionRequest,
} from "@agentclientprotocol/sdk";

interface SessionState {
  cancelled: boolean;
}

const sessions = new Map<string, SessionState>();
let sessionCounter = 0;

// Forward-declare so closures capture the initialized connection.
// eslint-disable-next-line prefer-const
let conn: AgentSideConnection;

 
const agent: Agent = {
  async initialize(_req: InitializeRequest) {
    return {
      protocolVersion: 1,
      agentCapabilities: {
        loadSession: false,
        mcpCapabilities: {},
        promptCapabilities: {
          audio: false,
          embeddedContext: false,
          image: false,
        },
      },
      authMethods: [],
    };
  },

  async newSession(_req: NewSessionRequest) {
    const sessionId = `mock-session-${++sessionCounter}`;
    sessions.set(sessionId, { cancelled: false });
    return { sessionId };
  },

  async loadSession(_req: LoadSessionRequest) {
    return {};
  },

  async authenticate(_req: AuthenticateRequest) {
    return {};
  },

  async setSessionMode(_req: SetSessionModeRequest) {
    return {};
  },

  async setSessionConfigOption(_req: SetSessionConfigOptionRequest) {
    return { configOptions: [] };
  },

  async prompt(req: PromptRequest) {
    const st = sessions.get(req.sessionId);
    if (!st) return { stopReason: "refusal" as const };
    st.cancelled = false;

    const firstBlock = req.prompt?.[0] as { type?: string; text?: string } | undefined;
    const marker = firstBlock?.type === "text" ? firstBlock.text : "";

    if (marker === "FS_READ") {
      // Exercise client-side fs handler round-trip.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const res = await (conn as any).readTextFile({
        sessionId: req.sessionId,
        path: "mock.txt",
      });
      await conn.sessionUpdate({
        sessionId: req.sessionId,
        update: {
          sessionUpdate: "agent_message_chunk",
          content: { type: "text", text: `got:${res.content}` },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any,
      });
      return { stopReason: "end_turn" as const };
    }

    // Default: emit 5 ticks, respecting cancellation.
    for (let i = 0; i < 5; i++) {
      if (st.cancelled) return { stopReason: "cancelled" as const };
      await conn.sessionUpdate({
        sessionId: req.sessionId,
        update: {
          sessionUpdate: "agent_message_chunk",
          content: { type: "text", text: `chunk${i}` },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any,
      });
      await new Promise((r) => setTimeout(r, 50));
    }
    return { stopReason: "end_turn" as const };
  },

  async cancel(req: CancelNotification) {
    const st = sessions.get(req.sessionId);
    if (st) st.cancelled = true;
  },
};

// Wrap Bun's stdio in real WebStream primitives. Bun.stdout.writer() returns a
// FileSink (not a WritableStream), so we adapt it; Bun.stdin.stream() already
// produces a ReadableStream<Uint8Array>.
const stdoutSink = Bun.stdout.writer();
const stdout = new WritableStream<Uint8Array>({
  write(chunk) {
    stdoutSink.write(chunk);
    stdoutSink.flush();
  },
  close() {
    try {
      stdoutSink.end();
    } catch {
      /* noop */
    }
  },
});

const stream = ndJsonStream(stdout, Bun.stdin.stream());
// eslint-disable-next-line @typescript-eslint/no-explicit-any
conn = new AgentSideConnection(() => agent as any, stream);
await conn.closed;
