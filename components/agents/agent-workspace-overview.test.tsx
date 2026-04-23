jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "workspace.agentGridTitle": "Agent Pool",
      "workspace.agentGridDescription": "Live runtime inventory and status cards.",
      "workspace.emptyTitle": "No active agents yet",
      "workspace.emptyDescription": "Start your first agent from the command palette.",
      "workspace.emptyAction": "Spawn your first agent",
      "workspace.cardRuntime": "Runtime",
      "workspace.cardBudget": "Budget",
      "workspace.cardTurns": "Turns",
      "workspace.cardMemory": "Memory",
      "workspace.filterAll": "All",
      "workspace.filterRunning": "Running",
      "workspace.filterPaused": "Paused",
      "workspace.filterError": "Error",
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
      "status.running": "running",
      "status.paused": "paused",
      "status.failed": "failed",
      "overview.bridgeHealth": "Bridge Health",
      "overview.bridgeStatus": "Status: {status}",
      "overview.bridgeActive": "Active {count}",
      "overview.bridgeAvailable": "Available {count}",
      "overview.bridgeWarm": "Warm {count}",
      "runtime.available": "Available",
      "runtime.unavailable": "Unavailable",
      "runtime.providers": "Providers: {providers}",
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
    if (key === "workspace.cardBudgetValue") {
      return `$${values?.cost ?? 0} / $${values?.budget ?? 0}`;
    }
    if (key === "overview.bridgeStatus") {
      return `Status: ${values?.status ?? ""}`;
    }
    if (key === "overview.bridgeActive") {
      return `Active ${values?.count ?? 0}`;
    }
    if (key === "overview.bridgeAvailable") {
      return `Available ${values?.count ?? 0}`;
    }
    if (key === "overview.bridgeWarm") {
      return `Warm ${values?.count ?? 0}`;
    }
    if (key === "runtime.providers") {
      return `Providers: ${values?.providers ?? "-"}`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentWorkspaceOverview } from "./agent-workspace-overview";

describe("AgentWorkspaceOverview", () => {
  it("renders monitor diagnostics, runtime catalog, queue, and dispatch stats", () => {
    render(
      <AgentWorkspaceOverview
        activeTab="monitor"
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
          {
            id: "agent-2",
            taskTitle: "Plan roadmap",
            roleName: "Planner",
            status: "paused",
            runtime: "claude_code",
            provider: "anthropic",
            model: "sonnet",
            turns: 7,
            cost: 1.5,
            budget: 5,
            memoryStatus: "warming",
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
    expect(screen.getByText("Agent Pool")).toBeInTheDocument();
    expect(screen.getByText("Audit release")).toBeInTheDocument();
    expect(screen.getByText("running")).toBeInTheDocument();
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

  it("filters monitor cards by status tabs and shows badge counts", async () => {
    const user = userEvent.setup();

    render(
      <AgentWorkspaceOverview
        activeTab="monitor"
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
          {
            id: "agent-2",
            taskTitle: "Plan roadmap",
            roleName: "Planner",
            status: "paused",
            runtime: "claude_code",
            provider: "anthropic",
            model: "sonnet",
            turns: 7,
            cost: 1.5,
            budget: 5,
            memoryStatus: "warming",
          } as never,
          {
            id: "agent-3",
            taskTitle: "Repair failed bridge",
            roleName: "Operator",
            status: "failed",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 3,
            cost: 0.7,
            budget: 5,
            memoryStatus: "none",
          } as never,
        ]}
        pool={null}
        runtimeCatalog={null}
        bridgeHealth={null}
        dispatchStats={null}
      />,
    );

    expect(screen.getByRole("button", { name: "All 3" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Running 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Paused 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Error 1" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Error 1" }));

    expect(screen.getByText("Repair failed bridge")).toBeInTheDocument();
    expect(screen.queryByText("Audit release")).not.toBeInTheDocument();
    expect(screen.queryByText("Plan roadmap")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "All 3" }));

    expect(screen.getByText("Audit release")).toBeInTheDocument();
    expect(screen.getByText("Plan roadmap")).toBeInTheDocument();
  });
});
