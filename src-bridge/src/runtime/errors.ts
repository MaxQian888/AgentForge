import type { AgentRuntimeKey } from "../types.js";

export type RuntimeOperationName =
  | "fork"
  | "rollback"
  | "revert"
  | "getMessages"
  | "getDiff"
  | "executeCommand"
  | "interrupt"
  | "setModel";

export class UnsupportedOperationError extends Error {
  constructor(
    public readonly operation: RuntimeOperationName,
    public readonly runtime: AgentRuntimeKey,
  ) {
    super(`Runtime ${runtime} does not support ${operation}`);
    this.name = "UnsupportedOperationError";
  }
}
