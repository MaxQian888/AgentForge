jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { normalizeTeam, useTeamStore } from "./team-store";

describe("useTeamStore", () => {
  const fetchMock = jest.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
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

  it("passes the explicit project scope when listing team runs", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => [],
    } as Response);

    await useTeamStore.getState().fetchTeams("project-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/teams?projectId=project-1",
      expect.objectContaining({
        method: "GET",
      })
    );
  });
});
