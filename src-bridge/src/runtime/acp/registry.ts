// Spawn-command map for ACP-backed adapters. Mirrors `openclaw/acpx`
// `src/agent-registry.ts` and the spec §5.1 table. T6 reads this when
// constructing the per-task `AcpClient`.

/** The four adapters this phase migrates to ACP. */
export type AdapterId = "claude_code" | "codex" | "opencode" | "cursor";

/** Per-adapter spawn descriptor + capability hints. */
export interface AcpAdapterConfig {
  /** Executable to invoke (resolved via PATH). */
  command: string;
  /** Arguments passed to the executable. */
  args: readonly string[];
  /**
   * Env vars the adapter needs to authenticate. The dispatcher (T6)
   * verifies presence before spawning and surfaces a structured error
   * when missing.
   */
  envRequired: readonly string[];
  /**
   * Whether the agent advertises Cursor-flavored extension methods
   * (`cursor/ask_question`, `cursor/create_plan`, ...). T10 logs +
   * returns `-32601 Method not found` for them; full support is a
   * future spec.
   */
  cursorExtensions: boolean;
}

/**
 * Adapter spawn registry. Pinned package versions follow spec §5.1
 * exactly; bumping these is a deliberate change that should land
 * alongside an integration-test refresh.
 */
export const ACP_ADAPTERS = {
  claude_code: {
    command: "npx",
    args: ["-y", "@agentclientprotocol/claude-agent-acp@^0.28.0"],
    envRequired: ["ANTHROPIC_API_KEY"],
    cursorExtensions: false,
  },
  codex: {
    command: "npx",
    args: ["-y", "@zed-industries/codex-acp@^0.11.1"],
    envRequired: ["OPENAI_API_KEY"],
    cursorExtensions: false,
  },
  opencode: {
    command: "opencode",
    args: ["acp"],
    envRequired: [],
    cursorExtensions: false,
  },
  cursor: {
    command: "cursor-agent",
    args: ["acp"],
    envRequired: [],
    cursorExtensions: true,
  },
} as const satisfies Record<AdapterId, AcpAdapterConfig>;
