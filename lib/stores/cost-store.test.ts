import { useCostStore } from "./cost-store";

const mockGet = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({ get: mockGet }),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

beforeEach(() => {
  useCostStore.setState({
    projectCost: null,
    loading: false,
    error: null,
    velocity: [],
    velocityLoading: false,
    agentPerformance: [],
    performanceLoading: false,
  });
  mockGet.mockReset();
});

describe("useCostStore", () => {
  it("fetches project cost and stores result", async () => {
    const mockData = {
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

    mockGet.mockResolvedValueOnce({ data: mockData });

    await useCostStore.getState().fetchProjectCost("proj-1");

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/stats/cost?projectId=proj-1",
      { token: "test-token" }
    );
    expect(useCostStore.getState().projectCost).toEqual(mockData);
    expect(useCostStore.getState().loading).toBe(false);
    expect(useCostStore.getState().error).toBeNull();
  });

  it("sets error on fetch failure", async () => {
    mockGet.mockRejectedValueOnce(new Error("Network error"));

    await useCostStore.getState().fetchProjectCost("proj-1");

    expect(useCostStore.getState().error).toBe("Unable to load cost data");
    expect(useCostStore.getState().projectCost).toBeNull();
    expect(useCostStore.getState().loading).toBe(false);
  });

  it("skips fetch when no auth token", async () => {
    jest.requireMock("./auth-store").useAuthStore.getState = () => ({
      accessToken: null,
    });

    await useCostStore.getState().fetchProjectCost("proj-1");

    expect(mockGet).not.toHaveBeenCalled();

    jest.requireMock("./auth-store").useAuthStore.getState = () => ({
      accessToken: "test-token",
    });
  });

  it("normalizes velocity wrapper responses into chart points", async () => {
    mockGet.mockResolvedValueOnce({
      data: {
        points: [
          { period: "2026-03-28", tasksCompleted: 2, costUsd: 5.25 },
          { period: "2026-03-29", tasksCompleted: 1, costUsd: 1.5 },
        ],
        totalCompleted: 3,
        totalCostUsd: 6.75,
        avgPerDay: 1.5,
      },
    });

    await useCostStore.getState().fetchVelocity("proj-1");

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/stats/velocity?projectId=proj-1",
      { token: "test-token" }
    );
    expect(useCostStore.getState().velocity).toEqual([
      { period: "2026-03-28", tasksCompleted: 2, costUsd: 5.25 },
      { period: "2026-03-29", tasksCompleted: 1, costUsd: 1.5 },
    ]);
  });

  it("normalizes performance wrapper responses into workspace records", async () => {
    mockGet.mockResolvedValueOnce({
      data: {
        entries: [
          {
            bucketId: "planner",
            label: "Planner",
            runCount: 4,
            successRate: 0.75,
            avgCostUsd: 1.25,
            avgDurationMinutes: 18,
            totalCostUsd: 5,
          },
        ],
      },
    });

    await useCostStore.getState().fetchAgentPerformance("proj-1");

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/stats/agent-performance?projectId=proj-1",
      { token: "test-token" }
    );
    expect(useCostStore.getState().agentPerformance).toEqual([
      {
        bucketId: "planner",
        label: "Planner",
        runCount: 4,
        successRate: 0.75,
        avgCostUsd: 1.25,
        avgDurationMinutes: 18,
        totalCostUsd: 5,
      },
    ]);
  });
});
