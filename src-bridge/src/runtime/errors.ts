import type { AgentRuntimeKey } from "../types.js";

export type RuntimeOperationName =
  | "fork"
  | "rollback"
  | "revert"
  | "getMessages"
  | "getDiff"
  | "executeCommand"
  | "executeShell"
  | "setThinkingBudget"
  | "getMcpServerStatus"
  | "interrupt"
  | "setModel";

export class UnsupportedOperationError extends Error {
  constructor(
    public readonly operation: RuntimeOperationName,
    public readonly runtime: AgentRuntimeKey,
    public readonly supportState: "unsupported" | "degraded" = "unsupported",
    public readonly reasonCode = "unsupported_operation",
    message?: string,
  ) {
    super(message ?? `Runtime ${runtime} does not support ${operation}`);
    this.name = "UnsupportedOperationError";
  }
}
