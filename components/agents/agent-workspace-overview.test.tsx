jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "diagnostics.title": "Pool Diagnostics",
      "diagnostics.description": "Live pool health",
      "diagnostics.warmReuseRatio": "Warm Reuse Ratio",
      "diagnostics.poolHealth": "Pool Health",
      "diagnostics.agentDistribution": "Agent Distribution",
      "diagnostics.blockedQueuedReasons": "Blocked Reasons",
      "diagnostics.degraded": "Degraded",
      "diagnostics.healthy": "Healthy",
      "stats.outcomes": "Outcomes",
      "stats.queueDepth": "Queue Depth",
      "stats.blockedReasons": "Blocked Reasons",
      "stats.none": "No dispatch data",
      "pool.activeSlots": "Active Slots",
      "pool.availableSlots": "Available Slots",
      "pool.pausedSessions": "Paused Sessions",
      "pool.warmSlots": "Warm Slots",
      "pool.queuedAdmissions": "Queued Admissions",
      "queue.task": "Task",
      "queue.runtime": "Runtime",
      "queue.priority": "Priority",
      "queue.status": "Status",
      "queue.reason": "Reason",
      "priority.high": "high",
      "dispatchStatus.started": "Started",
      "guardrail.budget": "budget",
    };
    if (key === "diagnostics.warmActive") {
      return `${values?.warm ?? 0}/${values?.active ?? 0} warm`;
    }
    if (key === "diagnostics.slotsAvailable") {
      return `${values?.count ?? 0} slots available`;
    }
    if (key === "stats.medianWait") {
      return `${values?.seconds ?? 0}s median wait`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { AgentWorkspaceOverview } from "./agent-workspace-overview";

describe("AgentWorkspaceOverview", () => {
  it("renders monitor diagnostics, runtime catalog, queue, and dispatch stats", () => {
    render(
      <AgentWorkspaceOverview
        activeTab="monitor"
        agents={[
          {
            id: "agent-1",
            status: "running",
            priority: 20,
          } as never,
          {
            id: "agent-2",
            status: "paused",
            priority: 10,
          } as never,
        ]}
        pool={{
          active: 2,
          max: 4,
          available: 2,
          pausedResumable: 1,
          queued: 1,
          warm: 1,
          degraded: true,
          queue: [
            {
              entryId: "queue-1",
              projectId: "project-1",
              taskId: "task-9",
              memberId: "member-1",
              runtime: "codex",
              provider: "openai",
              priority: 20,
              status: "blocked",
              reason: "budget",
              createdAt: "2026-03-30T11:00:00.000Z",
              updatedAt: "2026-03-30T11:00:00.000Z",
            },
          ],
        }}
        runtimeCatalog={{
          defaultRuntime: "codex",
          defaultSelection: {
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
          },
          runtimes: [
            {
              runtime: "codex",
              label: "Codex",
              defaultProvider: "openai",
              compatibleProviders: ["openai"],
              defaultModel: "gpt-5.4",
              modelOptions: ["gpt-5.4", "o3"],
              available: false,
              diagnostics: [{ code: "missing_cli", message: "CLI missing", blocking: true }],
              supportedFeatures: ["reasoning", "fork"],
            },
          ],
        }}
        bridgeHealth={{
          status: "degraded",
          lastCheck: "2026-03-30T11:00:00.000Z",
          pool: { active: 2, available: 2, warm: 1 },
        }}
        dispatchStats={{
          outcomes: { started: 3 },
          blockedReasons: { budget: 2 },
          queueDepth: 4,
          medianWaitSeconds: 12,
        }}
      />,
    );

    expect(screen.getByText("Bridge Health")).toBeInTheDocument();
    expect(screen.getByText(/Status: degraded/)).toBeInTheDocument();
    expect(screen.getByText("Pool Diagnostics")).toBeInTheDocument();
    expect(screen.getByText("1/2 warm")).toBeInTheDocument();
    expect(screen.getByText("Degraded")).toBeInTheDocument();
    expect(screen.getByText("Codex")).toBeInTheDocument();
    expect(screen.getByText("CLI missing")).toBeInTheDocument();
    expect(screen.getByText("task-9")).toBeInTheDocument();
    expect(screen.getByText("Started: 3")).toBeInTheDocument();
    expect(screen.getByText("budget: 2")).toBeInTheDocument();
    expect(screen.getByText("12s median wait")).toBeInTheDocument();
  });

  it("renders a dispatch empty state when no stats are available", () => {
    render(
      <AgentWorkspaceOverview
        activeTab="dispatch"
        agents={[]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
      />,
    );

    expect(screen.getByText("No dispatch data")).toBeInTheDocument();
  });
});
