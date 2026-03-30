jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import {
  getTeamStrategyLabel,
  normalizeTeam,
  useTeamStore,
} from "./team-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

function mockJsonResponse(data: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => data,
  } as Response;
}

describe("useTeamStore", () => {
  const fetchMock = jest.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    useTeamStore.setState({
      teams: [],
      loading: false,
      error: null,
      loadingById: {},
      errorById: {},
    });
  });

  it("normalizes runtime identity from team summaries", () => {
    const team = normalizeTeam({
      id: "team-1",
      projectId: "project-1",
      taskId: "task-1",
      taskTitle: "Finish provider support",
      name: "Runtime-complete team",
      status: "planning",
      strategy: "plan-code-review",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      coderRuns: [],
      totalBudgetUsd: 10,
      totalSpentUsd: 1.5,
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:05:00.000Z",
    });

    expect(team).toEqual(
      expect.objectContaining({
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
      })
    );
  });

  it("sends runtime/provider/model when starting a team", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({
        id: "team-1",
        projectId: "project-1",
        taskId: "task-1",
        taskTitle: "Finish provider support",
        name: "Runtime-complete team",
        status: "planning",
        strategy: "plan-code-review",
        runtime: "opencode",
        provider: "opencode",
        model: "opencode-default",
        coderRuns: [],
        totalBudgetUsd: 10,
        totalSpentUsd: 0,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:00.000Z",
      }),
    } as Response);

    await useTeamStore.getState().startTeam("task-1", "member-1", {
      runtime: "opencode",
      provider: "opencode",
      model: "opencode-default",
      totalBudgetUsd: 10,
      strategy: "plan-code-review",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/teams/start",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          taskId: "task-1",
          memberId: "member-1",
          strategy: "plan-code-review",
          totalBudgetUsd: 10,
          runtime: "opencode",
          provider: "opencode",
          model: "opencode-default",
        }),
      })
    );
  });

  it("normalizes the legacy planner_coder_reviewer strategy alias", () => {
    const team = normalizeTeam({
      id: "team-legacy",
      projectId: "project-1",
      taskId: "task-1",
      taskTitle: "Legacy team",
      name: "Legacy team",
      status: "planning",
      strategy: "planner_coder_reviewer",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      coderRuns: [],
      totalBudgetUsd: 10,
      totalSpentUsd: 0,
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:05:00.000Z",
    });

    expect(team.strategy).toBe("plan-code-review");
  });

  it("maps strategy labels and unknown strategies", () => {
    expect(getTeamStrategyLabel("planner-coder-reviewer")).toBe(
      "Planner → Coder → Reviewer",
    );
    expect(getTeamStrategyLabel("wave-based")).toBe("Wave Based");
    expect(getTeamStrategyLabel("pipeline")).toBe("Pipeline");
    expect(getTeamStrategyLabel("swarm")).toBe("Swarm");
    expect(getTeamStrategyLabel("custom")).toBe("Unknown strategy");
  });

  it("normalizes coder runs, default status, and fallback timestamps", () => {
    jest.useFakeTimers().setSystemTime(new Date("2026-03-30T12:00:00.000Z"));

    const team = normalizeTeam({
      id: "team-2",
      projectId: "project-1",
      taskId: "task-2",
      name: "Fallback team",
      coderRuns: [{ id: "run-1" }, "run-2"],
      totalBudget: 7,
      totalSpent: 3,
    });

    expect(team).toEqual(
      expect.objectContaining({
        status: "pending",
        coderRunIds: ["run-1", "run-2"],
        totalBudget: 7,
        totalSpent: 3,
        createdAt: "2026-03-30T12:00:00.000Z",
        updatedAt: "2026-03-30T12:00:00.000Z",
      }),
    );

    jest.useRealTimers();
  });

  it("passes the explicit project scope when listing team runs", async () => {
    fetchMock.mockResolvedValueOnce(mockJsonResponse([]));

    await useTeamStore.getState().fetchTeams("project-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/teams?projectId=project-1",
      expect.objectContaining({
        method: "GET",
      })
    );
  });

  it("includes the status filter and normalizes fetched teams", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "team-1",
          projectId: "project-1",
          taskId: "task-1",
          taskTitle: "Status filter",
          name: "Status filter",
          status: "executing",
          strategy: "planner-coder-reviewer",
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          coderRunIds: ["run-1"],
          totalBudgetUsd: 9,
          totalSpentUsd: 2,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:05:00.000Z",
        },
      ]),
    );

    await useTeamStore.getState().fetchTeams("project-1", "executing");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/teams?projectId=project-1&status=executing",
      expect.anything(),
    );
    expect(useTeamStore.getState().teams).toEqual([
      expect.objectContaining({
        status: "executing",
        strategy: "plan-code-review",
        coderRunIds: ["run-1"],
      }),
    ]);
  });

  it("stores fetchTeams failures as retryable errors", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({ message: "Unable to reach backend" }, 503),
    );

    await useTeamStore.getState().fetchTeams("project-1");

    expect(useTeamStore.getState()).toMatchObject({
      loading: false,
      teams: [],
      error: "Unable to reach backend",
    });
  });

  it("fetches a single team and tracks per-team loading state", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        id: "team-7",
        projectId: "project-1",
        taskId: "task-7",
        taskTitle: "Detail",
        name: "Detail team",
        status: "reviewing",
        strategy: "pipeline",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        coderRuns: [{ id: "run-7" }],
        totalBudgetUsd: 12,
        totalSpentUsd: 6,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:05:00.000Z",
      }),
    );

    const team = await useTeamStore.getState().fetchTeam("team-7");

    expect(team).toEqual(
      expect.objectContaining({
        id: "team-7",
        coderRunIds: ["run-7"],
      }),
    );
    expect(useTeamStore.getState()).toMatchObject({
      teams: [expect.objectContaining({ id: "team-7" })],
      loadingById: { "team-7": false },
      errorById: { "team-7": null },
    });
  });

  it("captures per-team fetch failures without throwing", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({ message: "Missing team" }, 404),
    );

    await expect(useTeamStore.getState().fetchTeam("team-missing")).resolves.toBeNull();

    expect(useTeamStore.getState()).toMatchObject({
      loadingById: { "team-missing": false },
      errorById: { "team-missing": "Missing team" },
    });
  });

  it("cancels, retries, updates, deletes, and upserts teams", async () => {
    useTeamStore.setState({
      teams: [
        normalizeTeam({
          id: "team-1",
          projectId: "project-1",
          taskId: "task-1",
          taskTitle: "Original",
          name: "Original",
          status: "executing",
          strategy: "pipeline",
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          coderRunIds: ["run-1"],
          plannerRunId: "planner-1",
          totalBudgetUsd: 10,
          totalSpentUsd: 3,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:05:00.000Z",
        }),
      ],
      loading: false,
      error: null,
      loadingById: {},
      errorById: {},
    });
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "team-1",
          projectId: "project-1",
          taskId: "task-1",
          taskTitle: "Original",
          name: "Original",
          status: "cancelled",
          strategy: "pipeline",
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          coderRunIds: ["run-1"],
          totalBudgetUsd: 10,
          totalSpentUsd: 3,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:06:00.000Z",
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "team-1",
          projectId: "project-1",
          taskId: "task-1",
          taskTitle: "Original",
          name: "Original",
          status: "planning",
          strategy: "pipeline",
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          coderRunIds: ["run-1"],
          totalBudgetUsd: 10,
          totalSpentUsd: 3,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:07:00.000Z",
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "team-1",
          projectId: "project-1",
          taskId: "task-1",
          taskTitle: "Original",
          name: "Renamed",
          status: "planning",
          strategy: "pipeline",
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
          coderRunIds: ["run-1"],
          totalBudgetUsd: 15,
          totalSpentUsd: 3,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:08:00.000Z",
        }),
      )
      .mockResolvedValueOnce(mockJsonResponse({}));

    await useTeamStore.getState().cancelTeam("team-1");
    await useTeamStore.getState().retryTeam("team-1");
    const updated = await useTeamStore.getState().updateTeam("team-1", {
      name: "Renamed",
      totalBudgetUsd: 15,
    });
    await useTeamStore.getState().deleteTeam("team-1");

    useTeamStore.getState().upsertTeam({
      id: "team-1",
      projectId: "project-1",
      taskId: "task-1",
      taskTitle: "Original",
      name: "Preserved optional fields",
      status: "planning",
      strategy: "pipeline",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      coderRunIds: ["run-1"],
      totalBudget: 15,
      totalSpent: 3,
      errorMessage: "",
      createdAt: "2026-03-25T10:00:00.000Z",
      updatedAt: "2026-03-25T10:09:00.000Z",
      plannerRunId: undefined,
      reviewerRunId: undefined,
    });

    expect(updated).toEqual(
      expect.objectContaining({
        name: "Renamed",
        totalBudget: 15,
      }),
    );
    expect(useTeamStore.getState().teams).toEqual([
      expect.objectContaining({
        id: "team-1",
        name: "Preserved optional fields",
        plannerRunId: undefined,
      }),
    ]);
  });

  it("returns early without an access token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useTeamStore.getState().fetchTeams("project-1");
    await expect(useTeamStore.getState().fetchTeam("team-1")).resolves.toBeNull();
    await useTeamStore.getState().startTeam("task-1", "member-1");
    await useTeamStore.getState().cancelTeam("team-1");
    await useTeamStore.getState().retryTeam("team-1");
    await useTeamStore.getState().deleteTeam("team-1");
    await expect(useTeamStore.getState().updateTeam("team-1", {})).resolves.toBeNull();

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
