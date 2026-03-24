import type { AgentStatus } from "../types.js";
import type { ExecuteRequest } from "../types.js";

export type RuntimeStatus =
  | "starting"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled"
  | "budget_exceeded";

export class AgentRuntime {
  readonly taskId: string;
  readonly sessionId: string;
  readonly abortController: AbortController;
  readonly createdAt: number;
  status: RuntimeStatus;
  turnNumber: number;
  spentUsd: number;
  lastActivity: number;
  lastTool: string;
  request: ExecuteRequest | null;
  budgetWarningEmitted: boolean;

  constructor(taskId: string, sessionId: string) {
    this.taskId = taskId;
    this.sessionId = sessionId;
    this.abortController = new AbortController();
    this.createdAt = Date.now();
    this.status = "starting";
    this.turnNumber = 0;
    this.spentUsd = 0;
    this.lastActivity = Date.now();
    this.lastTool = "";
    this.request = null;
    this.budgetWarningEmitted = false;
  }

  bindRequest(request: ExecuteRequest): void {
    this.request = { ...request };
  }

  cancel(nextStatus: Extract<RuntimeStatus, "paused" | "cancelled" | "budget_exceeded" | "failed"> = "cancelled"): void {
    this.abortController.abort(
      nextStatus === "paused"
        ? "paused_by_user"
        : nextStatus === "budget_exceeded"
          ? "budget_exceeded"
          : "cancelled_by_user",
    );
    this.status = nextStatus;
  }

  toStatus(): AgentStatus {
    const runtime = this.request?.runtime ?? "claude_code";
    const provider = this.request?.provider ?? "";
    const model = this.request?.model ?? "";
    return {
      task_id: this.taskId,
      state: this.status,
      turn_number: this.turnNumber,
      last_tool: this.lastTool,
      last_activity_ms: this.lastActivity,
      spent_usd: this.spentUsd,
      runtime,
      provider,
      model,
    };
  }
}
