import type {
  AgentStatus,
  ExecuteRequest,
  RuntimeContinuityState,
} from "../types.js";
import {
  serializeCostAccounting,
  type CostAccountingSnapshot,
} from "../cost/accounting.js";

export interface ClaudeQueryControl {
  interrupt?: () => Promise<void>;
  setModel?: (model?: string) => Promise<void>;
  setMaxThinkingTokens?: (maxThinkingTokens: number | null) => Promise<void>;
  rewindFiles?: (
    userMessageId: string,
    options?: { dryRun?: boolean },
  ) => Promise<{ canRewind: boolean; error?: string }>;
  mcpServerStatus?: () => Promise<unknown>;
}

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
  continuity: RuntimeContinuityState | null;
  structuredOutput: Record<string, unknown> | null;
  claudeQuery: ClaudeQueryControl | null;
  budgetWarningEmitted: boolean;
  costAccounting: CostAccountingSnapshot | null;

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
    this.continuity = null;
    this.structuredOutput = null;
    this.claudeQuery = null;
    this.budgetWarningEmitted = false;
    this.costAccounting = null;
  }

  bindRequest(request: ExecuteRequest): void {
    this.request = { ...request };
  }

  applyCostAccounting(snapshot: CostAccountingSnapshot): void {
    this.costAccounting = snapshot;
    this.spentUsd = snapshot.totalCostUsd;
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
    const roleId = this.request?.role_config?.role_id;
    const teamId = this.request?.team_id;
    const teamRole = this.request?.team_role;
    const continuityMatchesRuntime = this.continuity?.runtime === runtime;
    const resumeReady = continuityMatchesRuntime ? this.continuity?.resume_ready : undefined;
    const resumeBlockedReason = continuityMatchesRuntime
      ? this.continuity?.blocking_reason
      : undefined;
    const activeHooks = this.request?.hooks_config?.hooks.map((hook) => hook.hook);
    const subagentCount = this.request?.agents
      ? Object.keys(this.request.agents).length
      : undefined;
    const liveControls =
      runtime === "claude_code" && this.claudeQuery
        ? {
            interrupt: typeof this.claudeQuery.interrupt === "function" || undefined,
            set_model: typeof this.claudeQuery.setModel === "function" || undefined,
            set_thinking_budget:
              typeof this.claudeQuery.setMaxThinkingTokens === "function" || undefined,
            mcp_status:
              typeof this.claudeQuery.mcpServerStatus === "function" || undefined,
          }
        : undefined;
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
      role_id: roleId,
      team_id: teamId,
      team_role: teamRole,
      resume_ready: resumeReady,
      resume_blocked_reason: resumeBlockedReason,
      structured_output: this.structuredOutput ?? undefined,
      thinking_enabled: this.request?.thinking_config?.enabled,
      file_checkpointing: this.request?.file_checkpointing,
      active_hooks: activeHooks && activeHooks.length > 0 ? activeHooks : undefined,
      subagent_count: subagentCount,
      live_controls:
        liveControls && Object.values(liveControls).some(Boolean)
          ? liveControls
          : undefined,
      cost_accounting: serializeCostAccounting(this.costAccounting),
    };
  }
}
