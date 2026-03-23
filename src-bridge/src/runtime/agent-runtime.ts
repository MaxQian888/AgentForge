import type { AgentStatus } from "../types.js";

export type RuntimeStatus = "starting" | "running" | "paused" | "completed" | "failed";

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
  }

  cancel(): void {
    this.abortController.abort();
    this.status = "failed";
  }

  toStatus(): AgentStatus {
    return {
      task_id: this.taskId,
      state: this.status,
      turn_number: this.turnNumber,
      last_tool: this.lastTool,
      last_activity_ms: this.lastActivity,
      spent_usd: this.spentUsd,
    };
  }
}
