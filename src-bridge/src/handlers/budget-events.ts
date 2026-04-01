import type { AgentRuntime } from "../runtime/agent-runtime.js";
import type { AgentEvent } from "../types.js";

type BudgetAlertRequest = {
  task_id: string;
  session_id: string;
  budget_usd: number;
  warn_threshold?: number;
};

type EventSink = {
  send(event: AgentEvent): void;
};

export function emitBudgetAlertIfNeeded(
  runtime: AgentRuntime,
  streamer: EventSink,
  req: BudgetAlertRequest,
  now: () => number,
): void {
  const budget = typeof req.budget_usd === "number" ? req.budget_usd : 0;
  if (runtime.budgetWarningEmitted || budget <= 0) {
    return;
  }

  const thresholdRatio = req.warn_threshold ?? 0.8;
  if (runtime.spentUsd < budget * thresholdRatio) {
    return;
  }

  runtime.budgetWarningEmitted = true;
  streamer.send({
    task_id: req.task_id,
    session_id: req.session_id,
    timestamp_ms: now(),
    type: "budget_alert",
    data: {
      cost_usd: runtime.spentUsd,
      budget_remaining_usd: Math.max(budget - runtime.spentUsd, 0),
      threshold_ratio: thresholdRatio,
      threshold_percent: Math.round(thresholdRatio * 100),
      turn_number: runtime.turnNumber,
    },
  });
}
