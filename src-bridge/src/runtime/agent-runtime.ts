import type {
  AgentStatus,
  ExecuteRequest,
  RoleConfig,
  RuntimeContinuityState,
} from "../types.js";
import {
  serializeCostAccounting,
  type CostAccountingSnapshot,
} from "../cost/accounting.js";
import type { AcpRuntimeAdapter } from "./acp/index.js";

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
  budgetWarningEmitted: boolean;
  costAccounting: CostAccountingSnapshot | null;

  // Role enforcement limits
  maxTurnsLimit: number;
  apiCallCount: number;
  executionStartMs: number;
  turnLimitWarningEmitted: boolean;

  // ACP adapter — set during execute() for ACP-backed runtimes; null for legacy paths
  acpAdapter: AcpRuntimeAdapter | null;

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
    this.budgetWarningEmitted = false;
    this.costAccounting = null;
    this.maxTurnsLimit = 0;
    this.apiCallCount = 0;
    this.executionStartMs = Date.now();
    this.turnLimitWarningEmitted = false;
    this.acpAdapter = null;
  }

  bindRequest(request: ExecuteRequest): void {
    this.request = { ...request };
  }

  applyCostAccounting(snapshot: CostAccountingSnapshot): void {
    this.costAccounting = snapshot;
    this.spentUsd = snapshot.totalCostUsd;
  }

  /** Check if the agent has exceeded its turn limit. */
  isTurnLimitExceeded(): boolean {
    return this.maxTurnsLimit > 0 && this.turnNumber >= this.maxTurnsLimit;
  }

  /** Apply role enforcement limits from config. */
  applyRoleLimits(config: RoleConfig | undefined): void {
    if (!config) return;
    if (config.max_turns > 0) this.maxTurnsLimit = config.max_turns;
  }

  /** Increment API call count. */
  recordApiCall(): void {
    this.apiCallCount++;
  }

  /** Get execution duration in ms. */
  executionDurationMs(): number {
    return Date.now() - this.executionStartMs;
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
    // Capability-driven live_controls: use the ACP adapter's liveControls flags
    // when one is present (all 5 adapters now route through ACP exclusively).
    let liveControls: Record<string, true | undefined> | undefined;
    if (this.acpAdapter) {
      const lc = this.acpAdapter.liveControls;
      liveControls = {
        interrupt: true,
        set_model: lc.setModel || undefined,
        set_thinking_budget: lc.setThinkingBudget || undefined,
        mcp_status: lc.mcpServerStatus || undefined,
      };
    }
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
      ...(this.maxTurnsLimit > 0
        ? {
            role_enforcement: {
              max_turns: this.maxTurnsLimit,
              current_turn: this.turnNumber,
              turns_remaining: Math.max(0, this.maxTurnsLimit - this.turnNumber),
              api_calls: this.apiCallCount,
              execution_duration_ms: this.executionDurationMs(),
            },
          }
        : {}),
    } as AgentStatus;
  }
}
