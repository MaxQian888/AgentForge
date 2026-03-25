jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useAgentStore } from "./agent-store";

describe("useAgentStore", () => {
  const fetchMock = jest.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useAgentStore.setState({
      agents: [],
      agentOutputs: new Map(),
      loading: false,
    });
  });

  it("fetches canonical agent summaries and normalizes them for the UI store", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => [
        {
          id: "run-1",
          taskId: "task-1",
          taskTitle: "Implement orchestration",
          memberId: "member-1",
          roleId: "planner",
          roleName: "Planner",
          status: "paused",
          runtime: "codex",
          provider: "codex",
          model: "gpt-5-codex",
          turnCount: 4,
          costUsd: 1.25,
          budgetUsd: 5,
          worktreePath: "D:/Project/AgentForge/data/worktrees/project/task-1",
          branchName: "agent/task-1",
          sessionId: "session-1",
          lastActivityAt: "2026-03-24T10:00:00Z",
          startedAt: "2026-03-24T09:00:00Z",
          createdAt: "2026-03-24T09:00:00Z",
          canResume: true,
          memoryStatus: "available",
        },
      ],
    } as Response);

    await useAgentStore.getState().fetchAgents();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/agents",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );

    expect(useAgentStore.getState().agents).toEqual([
      expect.objectContaining({
        id: "run-1",
        taskId: "task-1",
        taskTitle: "Implement orchestration",
        roleName: "Planner",
        status: "paused",
        runtime: "codex",
        turns: 4,
        cost: 1.25,
        budget: 5,
        worktreePath: "D:/Project/AgentForge/data/worktrees/project/task-1",
        sessionId: "session-1",
        canResume: true,
        memoryStatus: "available",
      }),
    ]);
  });

  it("uses the canonical spawn and resume endpoints with camelCase payloads", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({
          task: {
            id: "task-1",
          },
          dispatch: {
            status: "started",
            run: {
              id: "run-1",
              taskId: "task-1",
              taskTitle: "Implement orchestration",
              memberId: "member-1",
              roleId: "planner",
              roleName: "Planner",
              status: "running",
              runtime: "codex",
              provider: "codex",
              model: "gpt-5-codex",
              turnCount: 1,
              costUsd: 0.1,
              budgetUsd: 5,
              worktreePath: "D:/Project/AgentForge/data/worktrees/project/task-1",
              branchName: "agent/task-1",
              sessionId: "session-1",
              lastActivityAt: "2026-03-24T10:00:00Z",
              startedAt: "2026-03-24T09:00:00Z",
              createdAt: "2026-03-24T09:00:00Z",
              canResume: false,
              memoryStatus: "warming",
            },
          },
        }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          id: "run-1",
          status: "running",
          canResume: false,
        }),
      } as Response);

    await useAgentStore.getState().spawnAgent("task-1", "member-1", {
      runtime: "codex",
      provider: "codex",
      model: "gpt-5-codex",
      roleId: "planner",
      maxBudgetUsd: 5,
    });

    await useAgentStore.getState().resumeAgent("run-1");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "http://localhost:7777/api/v1/agents/spawn",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          taskId: "task-1",
          memberId: "member-1",
          runtime: "codex",
          provider: "codex",
          model: "gpt-5-codex",
          roleId: "planner",
          maxBudgetUsd: 5,
        }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "http://localhost:7777/api/v1/agents/run-1/resume",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({}),
      }),
    );
  });

  it("keeps pool queue state when spawn is accepted into the queue", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: async () => ({
        task: {
          id: "task-2",
        },
        dispatch: {
          status: "queued",
          reason: "agent pool is at capacity",
          queue: {
            entryId: "queue-1",
            projectId: "project-1",
            taskId: "task-2",
            memberId: "member-2",
            status: "queued",
            reason: "agent pool is at capacity",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
            budgetUsd: 5,
            createdAt: "2026-03-25T10:00:00Z",
            updatedAt: "2026-03-25T10:00:00Z",
          },
        },
      }),
    } as Response);

    useAgentStore.setState({
      agents: [],
      agentOutputs: new Map(),
      pool: {
        active: 1,
        max: 2,
        available: 1,
        pausedResumable: 0,
        queued: 0,
        warm: 1,
        degraded: false,
        queue: [],
      },
      loading: false,
    });

    await useAgentStore.getState().spawnAgent("task-2", "member-2", {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      maxBudgetUsd: 5,
    });

    expect(useAgentStore.getState().agents).toHaveLength(0);
    expect(useAgentStore.getState().pool).toEqual(
      expect.objectContaining({
        queued: 1,
        queue: [
          expect.objectContaining({
            entryId: "queue-1",
            taskId: "task-2",
            status: "queued",
          }),
        ],
      }),
    );
  });
});
