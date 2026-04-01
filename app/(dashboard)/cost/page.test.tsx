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
};

jest.mock("@/lib/stores/cost-store", () => ({
  useCostStore: (selector: (s: MockCostState) => unknown) =>
    selector(mockCostState),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector(mockDashboardState),
}));

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
});
