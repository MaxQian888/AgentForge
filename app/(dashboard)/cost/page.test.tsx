import { render, screen } from "@testing-library/react";
import type { ProjectCostSummary } from "@/lib/stores/cost-store";
import CostPage from "./page";

type MockCostState = {
  projectCost: ProjectCostSummary | null;
  loading: boolean;
  error: string | null;
  fetchProjectCost: (projectId: string) => Promise<void>;
  velocity: unknown[];
  velocityLoading: boolean;
  agentPerformance: unknown[];
  performanceLoading: boolean;
  fetchVelocity: (projectId: string) => Promise<void>;
  fetchAgentPerformance: (projectId: string) => Promise<void>;
};

const mockCostState: MockCostState = {
  projectCost: null,
  loading: false,
  error: null,
  fetchProjectCost: jest.fn(async () => undefined),
  velocity: [],
  velocityLoading: false,
  agentPerformance: [],
  performanceLoading: false,
  fetchVelocity: jest.fn(async () => undefined),
  fetchAgentPerformance: jest.fn(async () => undefined),
};

const mockDashboardState = {
  selectedProjectId: null as string | null,
  projects: [] as Array<{ id: string; name: string }>,
};

jest.mock("@/lib/stores/cost-store", () => ({
  useCostStore: (selector: (s: MockCostState) => unknown) =>
    selector(mockCostState),
}));

jest.mock("@/lib/stores/dashboard-store", () => {
  const useDashboardStore = (
    selector: (s: Record<string, unknown>) => unknown,
  ) => selector(mockDashboardState);
  (
    useDashboardStore as unknown as {
      setState: (patch: Record<string, unknown>) => void;
    }
  ).setState = (patch: Record<string, unknown>) => {
    Object.assign(mockDashboardState, patch);
  };
  return { useDashboardStore };
});

jest.mock("@/components/cost/cost-chart", () => ({
  CostChart: () => <div data-testid="cost-chart" />,
}));

jest.mock("@/components/cost/velocity-chart", () => ({
  VelocityChart: ({ data }: { data: unknown[] }) =>
    data.length === 0 ? (
      <div>No velocity data available yet.</div>
    ) : (
      <div data-testid="velocity-chart" />
    ),
}));

jest.mock("@/components/cost/agent-performance-table", () => ({
  AgentPerformanceTable: ({ data }: { data: unknown[] }) =>
    data.length === 0 ? (
      <div>No execution-bucket performance data available yet.</div>
    ) : (
      <div data-testid="agent-performance-table" />
    ),
}));

jest.mock("@/components/cost/spending-trend-chart", () => ({
  SpendingTrendChart: () => <div data-testid="spending-trend-chart" />,
}));

jest.mock("@/components/cost/budget-allocation-chart", () => ({
  BudgetAllocationChart: () => <div data-testid="budget-allocation-chart" />,
}));

jest.mock("@/components/cost/agent-cost-bar-chart", () => ({
  AgentCostBarChart: () => <div data-testid="agent-cost-bar-chart" />,
}));

jest.mock("@/components/cost/budget-forecast-card", () => ({
  BudgetForecastCard: () => <div data-testid="budget-forecast-card" />,
}));

jest.mock("@/components/cost/cost-breakdown-table", () => ({
  CostBreakdownTable: ({ data }: { data: unknown[] }) => (
    <div data-testid="cost-breakdown-table" data-count={data.length} />
  ),
}));

jest.mock("@/components/cost/cost-csv-export", () => ({
  CostCsvExport: ({ data }: { data: unknown[] }) => (
    <button data-testid="cost-csv-export" data-count={data.length}>
      Export CSV
    </button>
  ),
}));

jest.mock("@/components/cost/overspending-alert", () => ({
  OverspendingAlertBanner: ({ alerts }: { alerts: unknown[] }) =>
    alerts.length === 0 ? null : (
      <div data-testid="overspending-alerts" data-count={alerts.length} />
    ),
  deriveOverspendingAlerts: (
    items: Array<{
      id: string;
      scope: string;
      spentUsd: number;
      budgetUsd: number;
    }>,
  ) =>
    items
      .filter(
        (item) =>
          item.budgetUsd > 0 && item.spentUsd / item.budgetUsd >= 0.8,
      )
      .map((item) => ({
        ...item,
        severity:
          item.spentUsd / item.budgetUsd >= 1 ? "critical" : "warning",
      })),
}));

jest.mock("@/components/cost/cost-project-filter", () => ({
  CostProjectFilter: ({
    projects,
    selectedProjectId,
  }: {
    projects: Array<{ id: string; name: string }>;
    selectedProjectId: string | null;
  }) => (
    <div
      data-testid="cost-project-filter"
      data-count={projects.length}
      data-selected={selectedProjectId ?? ""}
    />
  ),
}));

describe("CostPage", () => {
  beforeEach(() => {
    Object.assign(mockCostState, {
      projectCost: null,
      loading: false,
      error: null,
      fetchProjectCost: jest.fn(async () => undefined),
      velocity: [],
      velocityLoading: false,
      agentPerformance: [],
      performanceLoading: false,
      fetchVelocity: jest.fn(async () => undefined),
      fetchAgentPerformance: jest.fn(async () => undefined),
    });
    mockDashboardState.selectedProjectId = null;
    mockDashboardState.projects = [];
  });

  it("renders the cost overview heading and summary cards", () => {
    mockDashboardState.selectedProjectId = "proj-1";
    mockCostState.projectCost = {
      totalCostUsd: 12.5,
      totalInputTokens: 100000,
      totalOutputTokens: 50000,
      totalCacheReadTokens: 20000,
      totalTurns: 42,
      runCount: 3,
      activeAgents: 2,
      sprintCosts: [],
      taskCosts: [],
      dailyCosts: [],
      budgetSummary: null,
      costCoverage: {
        totalRunCount: 3,
        pricedRunCount: 2,
        authoritativeRunCount: 1,
        estimatedRunCount: 1,
        planIncludedRunCount: 0,
        unpricedRunCount: 1,
        totalCostUsd: 12.5,
        authoritativeCostUsd: 8,
        estimatedCostUsd: 4.5,
        hasCoverageGap: true,
      },
      runtimeBreakdown: [
        {
          runtime: "claude_code",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          runCount: 1,
          pricedRunCount: 1,
          authoritativeRunCount: 1,
          estimatedRunCount: 0,
          planIncludedRunCount: 0,
          unpricedRunCount: 0,
          totalCostUsd: 8,
        },
      ],
      periodRollups: {
        today: { costUsd: 1, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last7Days: { costUsd: 4, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last30Days: { costUsd: 12.5, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 3 },
      },
    };

    render(<CostPage />);
    expect(screen.getByText("Cost Overview")).toBeInTheDocument();
    expect(screen.getByText("Total Spend")).toBeInTheDocument();
    expect(screen.getByText("Input Tokens")).toBeInTheDocument();
    expect(screen.getByText("Output Tokens")).toBeInTheDocument();
    expect(screen.getByText("Active Agents")).toBeInTheDocument();
    expect(screen.getByText("External Runtime Cost Coverage")).toBeInTheDocument();
    expect(screen.getByText("Runtime Cost Breakdown")).toBeInTheDocument();
  });

  it("shows an explicit project-selection message instead of zeroed analytics", () => {
    render(<CostPage />);

    expect(screen.getByText("Cost Overview")).toBeInTheDocument();
    expect(
      screen.getByText("Select a project to view cost statistics."),
    ).toBeInTheDocument();
  });

  it("keeps empty sections visible when the selected project has no data", () => {
    mockDashboardState.selectedProjectId = "proj-1";
    mockCostState.projectCost = {
      totalCostUsd: 0,
      totalInputTokens: 0,
      totalOutputTokens: 0,
      totalCacheReadTokens: 0,
      totalTurns: 0,
      runCount: 0,
      activeAgents: 0,
      sprintCosts: [],
      taskCosts: [],
      dailyCosts: [],
      budgetSummary: null,
      costCoverage: {
        totalRunCount: 0,
        pricedRunCount: 0,
        authoritativeRunCount: 0,
        estimatedRunCount: 0,
        planIncludedRunCount: 0,
        unpricedRunCount: 0,
        totalCostUsd: 0,
        authoritativeCostUsd: 0,
        estimatedCostUsd: 0,
        hasCoverageGap: false,
      },
      runtimeBreakdown: [],
      periodRollups: {
        today: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
        last7Days: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
        last30Days: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
      },
    };

    render(<CostPage />);

    expect(screen.getByText("No daily cost data available yet.")).toBeInTheDocument();
    expect(screen.getByText("No velocity data available yet.")).toBeInTheDocument();
    expect(
      screen.getByText("No execution-bucket performance data available yet."),
    ).toBeInTheDocument();
    expect(screen.getByText("No sprint cost data available yet.")).toBeInTheDocument();
    expect(screen.getByText("No per-task cost data available yet.")).toBeInTheDocument();
    expect(
      screen.getByText("No external runtime breakdown data available yet."),
    ).toBeInTheDocument();
  });

  it("surfaces runtime cost coverage and truthful gap messaging", () => {
    mockDashboardState.selectedProjectId = "proj-1";
    mockCostState.projectCost = {
      totalCostUsd: 12.5,
      totalInputTokens: 100000,
      totalOutputTokens: 50000,
      totalCacheReadTokens: 20000,
      totalTurns: 42,
      runCount: 3,
      activeAgents: 2,
      sprintCosts: [],
      taskCosts: [],
      dailyCosts: [],
      budgetSummary: null,
      costCoverage: {
        totalRunCount: 3,
        pricedRunCount: 2,
        authoritativeRunCount: 1,
        estimatedRunCount: 1,
        planIncludedRunCount: 0,
        unpricedRunCount: 1,
        totalCostUsd: 12.5,
        authoritativeCostUsd: 8,
        estimatedCostUsd: 4.5,
        hasCoverageGap: true,
      },
      runtimeBreakdown: [
        {
          runtime: "claude_code",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          runCount: 1,
          pricedRunCount: 1,
          authoritativeRunCount: 1,
          estimatedRunCount: 0,
          planIncludedRunCount: 0,
          unpricedRunCount: 0,
          totalCostUsd: 8,
        },
        {
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          runCount: 1,
          pricedRunCount: 1,
          authoritativeRunCount: 0,
          estimatedRunCount: 1,
          planIncludedRunCount: 0,
          unpricedRunCount: 0,
          totalCostUsd: 4.5,
        },
        {
          runtime: "opencode",
          provider: "opencode",
          model: "opencode-default",
          runCount: 1,
          pricedRunCount: 0,
          authoritativeRunCount: 0,
          estimatedRunCount: 0,
          planIncludedRunCount: 0,
          unpricedRunCount: 1,
          totalCostUsd: 0,
        },
      ],
      periodRollups: {
        today: { costUsd: 1, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last7Days: { costUsd: 4, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last30Days: { costUsd: 12.5, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 3 },
      },
    };

    render(<CostPage />);

    expect(screen.getByText("Authoritative Spend")).toBeInTheDocument();
    expect(screen.getByText("Estimated Spend")).toBeInTheDocument();
    expect(screen.getByText("Unpriced Runs")).toBeInTheDocument();
    expect(
      screen.getByText("Some runtime activity is outside truthful USD coverage."),
    ).toBeInTheDocument();
    expect(screen.getByText("claude_code")).toBeInTheDocument();
    expect(screen.getByText("gpt-5-codex")).toBeInTheDocument();
    expect(screen.getByText("opencode-default")).toBeInTheDocument();
  });

  it("renders forecast, trend, allocation, agent cost, breakdown, CSV export, and project filter", () => {
    mockDashboardState.selectedProjectId = "proj-1";
    mockDashboardState.projects = [
      { id: "proj-1", name: "Project One" },
      { id: "proj-2", name: "Project Two" },
    ];
    mockCostState.projectCost = {
      totalCostUsd: 10,
      totalInputTokens: 1,
      totalOutputTokens: 1,
      totalCacheReadTokens: 0,
      totalTurns: 1,
      runCount: 1,
      activeAgents: 1,
      sprintCosts: [],
      taskCosts: [
        {
          taskId: "t-1",
          taskTitle: "Ship feature X",
          agentRuns: 3,
          costUsd: 4,
          inputTokens: 10,
          outputTokens: 5,
          cacheReadTokens: 0,
        },
      ],
      dailyCosts: [
        { date: "2026-04-15", costUsd: 5 },
        { date: "2026-04-16", costUsd: 5 },
      ],
      budgetSummary: {
        allocated: 100,
        spent: 10,
        remaining: 90,
        thresholdStatus: "ok",
      },
      costCoverage: {
        totalRunCount: 1,
        pricedRunCount: 1,
        authoritativeRunCount: 1,
        estimatedRunCount: 0,
        planIncludedRunCount: 0,
        unpricedRunCount: 0,
        totalCostUsd: 10,
        authoritativeCostUsd: 10,
        estimatedCostUsd: 0,
        hasCoverageGap: false,
      },
      runtimeBreakdown: [
        {
          runtime: "claude_code",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          runCount: 1,
          pricedRunCount: 1,
          authoritativeRunCount: 1,
          estimatedRunCount: 0,
          planIncludedRunCount: 0,
          unpricedRunCount: 0,
          totalCostUsd: 10,
        },
      ],
      periodRollups: {
        today: { costUsd: 5, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last7Days: { costUsd: 10, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
        last30Days: { costUsd: 10, inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, turns: 1, runCount: 1 },
      },
    };

    render(<CostPage />);

    expect(screen.getByTestId("budget-forecast-card")).toBeInTheDocument();
    expect(screen.getByTestId("spending-trend-chart")).toBeInTheDocument();
    expect(screen.getByTestId("budget-allocation-chart")).toBeInTheDocument();
    expect(screen.getByTestId("agent-cost-bar-chart")).toBeInTheDocument();
    expect(screen.getByTestId("cost-breakdown-table")).toHaveAttribute(
      "data-count",
      "3",
    );
    expect(screen.getByTestId("cost-csv-export")).toBeInTheDocument();
    expect(screen.getByTestId("cost-project-filter")).toHaveAttribute(
      "data-count",
      "2",
    );
  });

  it("shows an overspending alert banner when spend exceeds budget", () => {
    mockDashboardState.selectedProjectId = "proj-1";
    mockCostState.projectCost = {
      totalCostUsd: 100,
      totalInputTokens: 1,
      totalOutputTokens: 1,
      totalCacheReadTokens: 0,
      totalTurns: 1,
      runCount: 1,
      activeAgents: 1,
      sprintCosts: [
        {
          sprintId: "s-1",
          sprintName: "Sprint Alpha",
          costUsd: 150,
          budgetUsd: 100,
          inputTokens: 0,
          outputTokens: 0,
        },
      ],
      taskCosts: [],
      dailyCosts: [],
      budgetSummary: null,
      costCoverage: null,
      runtimeBreakdown: [],
      periodRollups: {
        today: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
        last7Days: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
        last30Days: { costUsd: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, turns: 0, runCount: 0 },
      },
    };

    render(<CostPage />);

    expect(screen.getByTestId("overspending-alerts")).toHaveAttribute(
      "data-count",
      "1",
    );
  });
});
