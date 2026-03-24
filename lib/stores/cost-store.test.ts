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
      activeAgents: 2,
      sprintCosts: [],
      taskCosts: [],
      dailyCosts: [],
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

    // Restore token for other tests
    jest.requireMock("./auth-store").useAuthStore.getState = () => ({
      accessToken: "test-token",
    });
  });
});
