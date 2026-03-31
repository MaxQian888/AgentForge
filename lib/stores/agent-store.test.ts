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
      dispatchStats: null,
      dispatchHistoryByTask: {},
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

  it("fetches runtime catalog from the bridge API and caches the normalized result", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        default_runtime: "claude_code",
        runtimes: [
          {
            key: "claude_code",
            display_name: "Claude Code",
            default_provider: "anthropic",
            compatible_providers: ["anthropic"],
            default_model: "claude-sonnet-4-5",
            model_options: ["claude-sonnet-4-5", "claude-opus-4-1"],
            available: true,
            diagnostics: [],
            supported_features: ["structured_output", "interrupt"],
          },
          {
            key: "cursor",
            display_name: "Cursor Agent",
            default_provider: "cursor",
            compatible_providers: ["cursor"],
            default_model: "claude-sonnet-4-20250514",
            model_options: ["claude-sonnet-4-20250514", "gpt-4o"],
            available: true,
            diagnostics: [],
            supported_features: ["progress", "reasoning"],
          },
        ],
      }),
    } as Response);

    const first = await useAgentStore.getState().fetchRuntimeCatalog();
    const second = await useAgentStore.getState().fetchRuntimeCatalog();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(first).toEqual(
      expect.objectContaining({
        defaultRuntime: "claude_code",
        defaultSelection: expect.objectContaining({
          runtime: "claude_code",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
        }),
        runtimes: expect.arrayContaining([
          expect.objectContaining({
            runtime: "cursor",
            modelOptions: ["claude-sonnet-4-20250514", "gpt-4o"],
            supportedFeatures: ["progress", "reasoning"],
          }),
        ]),
      }),
    );
    expect(second).toEqual(first);
  });

  it("fetches bridge health summary and normalizes pool metrics", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        status: "ready",
        last_check: "2026-03-28T10:00:00Z",
        pool: {
          active: 2,
          available: 1,
          warm: 1,
        },
      }),
    } as Response);

    const health = await useAgentStore.getState().fetchBridgeHealth();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/bridge/health",
      expect.objectContaining({
        method: "GET",
      }),
    );
    expect(health).toEqual({
      status: "ready",
      lastCheck: "2026-03-28T10:00:00Z",
      pool: {
        active: 2,
        available: 1,
        warm: 1,
      },
    });
  });

  it("fetches dispatch preflight, history, and stats endpoints", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          admissionLikely: true,
          dispatchOutcomeHint: "started",
          poolActive: 1,
          poolAvailable: 1,
          poolQueued: 0,
        }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [
          {
            id: "attempt-1",
            projectId: "project-1",
            taskId: "task-1",
            memberId: "member-1",
            outcome: "queued",
            triggerSource: "manual",
            createdAt: "2026-03-28T10:00:00Z",
          },
        ],
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          outcomes: { started: 2, queued: 1 },
          blockedReasons: { budget: 1 },
          queueDepth: 3,
          medianWaitSeconds: 12,
        }),
      } as Response);

    const preflight = await useAgentStore.getState().fetchDispatchPreflight("project-1", "task-1", "member-1");
    const history = await useAgentStore.getState().fetchDispatchHistory("task-1");
    const stats = await useAgentStore.getState().fetchDispatchStats("project-1");

    expect(preflight).toEqual(
      expect.objectContaining({
        admissionLikely: true,
        dispatchOutcomeHint: "started",
      }),
    );
    expect(history).toEqual([
      expect.objectContaining({
        id: "attempt-1",
        outcome: "queued",
      }),
    ]);
    expect(stats).toEqual(
      expect.objectContaining({
        queueDepth: 3,
        medianWaitSeconds: 12,
      }),
    );
    expect(useAgentStore.getState().dispatchHistoryByTask["task-1"]).toHaveLength(1);
    expect(useAgentStore.getState().dispatchStats).toEqual(
      expect.objectContaining({
        queueDepth: 3,
      }),
    );
  });
});
