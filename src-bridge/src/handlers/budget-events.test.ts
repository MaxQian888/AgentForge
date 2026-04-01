import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import { emitBudgetAlertIfNeeded } from "./budget-events.js";

describe("emitBudgetAlertIfNeeded", () => {
  test("emits a budget_alert once when spend crosses the threshold", () => {
    const runtime = new AgentRuntime("task-123", "session-123");
    runtime.turnNumber = 2;
    runtime.spentUsd = 4.2;
    const events: Array<{ type: string; data: unknown }> = [];

    emitBudgetAlertIfNeeded(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      {
        task_id: "task-123",
        session_id: "session-123",
        budget_usd: 5,
        warn_threshold: 0.8,
      },
      () => 1_700_000_000_000,
    );

    emitBudgetAlertIfNeeded(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      {
        task_id: "task-123",
        session_id: "session-123",
        budget_usd: 5,
        warn_threshold: 0.8,
      },
      () => 1_700_000_000_001,
    );

    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({
      type: "budget_alert",
      data: {
        cost_usd: 4.2,
        threshold_ratio: 0.8,
        threshold_percent: 80,
        turn_number: 2,
      },
    });
    expect((events[0]?.data as { budget_remaining_usd?: number }).budget_remaining_usd).toBeCloseTo(0.8, 6);
    expect(runtime.budgetWarningEmitted).toBe(true);
  });

  test("does not emit before the threshold is reached", () => {
    const runtime = new AgentRuntime("task-123", "session-123");
    runtime.spentUsd = 1.5;
    const events: Array<{ type: string; data: unknown }> = [];

    emitBudgetAlertIfNeeded(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      {
        task_id: "task-123",
        session_id: "session-123",
        budget_usd: 5,
      },
      () => 1_700_000_000_000,
    );

    expect(events).toHaveLength(0);
    expect(runtime.budgetWarningEmitted).toBe(false);
  });
});
