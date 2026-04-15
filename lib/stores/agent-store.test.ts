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
            interaction_capabilities: {
              lifecycle: {
                interrupt: {
                  state: "supported",
                  message: "Interrupt is available",
                },
              },
            },
            providers: [
              {
                provider: "anthropic",
                connected: true,
                default_model: "claude-sonnet-4-5",
                model_options: ["claude-sonnet-4-5", "claude-opus-4-1"],
                auth_required: false,
                auth_methods: ["api_key"],
              },
            ],
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
            launch_contract: {
              prompt_transport: "positional",
              output_mode: "stream-json",
              supported_output_modes: ["text", "json", "stream-json"],
              supported_approval_modes: ["default", "ask", "plan", "yolo"],
              additional_directories: false,
              env_overrides: false,
            },
            lifecycle: {
              stage: "active",
            },
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
            runtime: "claude_code",
            interactionCapabilities: expect.objectContaining({
              lifecycle: expect.objectContaining({
                interrupt: expect.objectContaining({
                  state: "supported",
                }),
              }),
            }),
            providers: [
              expect.objectContaining({
                provider: "anthropic",
                connected: true,
                defaultModel: "claude-sonnet-4-5",
                modelOptions: ["claude-sonnet-4-5", "claude-opus-4-1"],
                authRequired: false,
                authMethods: ["api_key"],
              }),
            ],
          }),
          expect.objectContaining({
            runtime: "cursor",
            modelOptions: ["claude-sonnet-4-20250514", "gpt-4o"],
            supportedFeatures: ["progress", "reasoning"],
            launchContract: expect.objectContaining({
              promptTransport: "positional",
              outputMode: "stream-json",
              supportedApprovalModes: ["default", "ask", "plan", "yolo"],
            }),
            lifecycle: expect.objectContaining({
              stage: "active",
            }),
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

    const preflight = await useAgentStore.getState().fetchDispatchPreflight("project-1", "task-1", "member-1", {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      roleId: "frontend-developer",
      budgetUsd: 7.5,
    });
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
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/projects/project-1/dispatch/preflight?taskId=task-1&memberId=member-1&runtime=codex&provider=openai&model=gpt-5-codex&roleId=frontend-developer&budgetUsd=7.5",
      expect.objectContaining({
        method: "GET",
      }),
    );
  });

  describe("streaming data methods", () => {
    beforeEach(() => {
      useAgentStore.setState({
        agentToolCalls: new Map(),
        agentToolResults: new Map(),
        agentReasoning: new Map(),
        agentFileChanges: new Map(),
        agentTodos: new Map(),
        agentPartialMessages: new Map(),
        agentPermissionRequests: new Map(),
      });
    });

    it("appendToolCall creates entry for new id and accumulates for existing", () => {
      const store = useAgentStore.getState();
      store.appendToolCall("a1", { toolName: "read_file", toolCallId: "tc-1" });
      expect(useAgentStore.getState().agentToolCalls.get("a1")).toEqual([
        { toolName: "read_file", toolCallId: "tc-1" },
      ]);

      store.appendToolCall("a1", { toolName: "write_file", turnNumber: 2 });
      expect(useAgentStore.getState().agentToolCalls.get("a1")).toHaveLength(2);
      expect(useAgentStore.getState().agentToolCalls.get("a1")![1]).toEqual(
        expect.objectContaining({ toolName: "write_file" }),
      );
    });

    it("appendToolResult creates entry for new id and accumulates for existing", () => {
      const store = useAgentStore.getState();
      store.appendToolResult("a1", { toolName: "read_file", toolCallId: "tc-1", output: "ok" });
      store.appendToolResult("a1", { toolName: "write_file", isError: true });

      const results = useAgentStore.getState().agentToolResults.get("a1")!;
      expect(results).toHaveLength(2);
      expect(results[0]).toEqual(expect.objectContaining({ toolName: "read_file", output: "ok" }));
      expect(results[1]).toEqual(expect.objectContaining({ isError: true }));
    });

    it("setReasoning replaces previous value", () => {
      const store = useAgentStore.getState();
      store.setReasoning("a1", "thinking...");
      expect(useAgentStore.getState().agentReasoning.get("a1")).toBe("thinking...");

      store.setReasoning("a1", "revised reasoning");
      expect(useAgentStore.getState().agentReasoning.get("a1")).toBe("revised reasoning");
    });

    it("appendFileChanges creates and accumulates file entries", () => {
      const store = useAgentStore.getState();
      store.appendFileChanges("a1", [{ path: "src/a.ts", changeType: "create" }]);
      store.appendFileChanges("a1", [{ path: "src/b.ts" }, { path: "src/c.ts" }]);

      const files = useAgentStore.getState().agentFileChanges.get("a1")!;
      expect(files).toHaveLength(3);
      expect(files[0]).toEqual({ path: "src/a.ts", changeType: "create" });
    });

    it("setTodos replaces the entire array", () => {
      const store = useAgentStore.getState();
      store.setTodos("a1", [{ id: "t1", content: "first", status: "pending" }]);
      expect(useAgentStore.getState().agentTodos.get("a1")).toHaveLength(1);

      store.setTodos("a1", [{ id: "t2", content: "second", status: "done" }]);
      const todos = useAgentStore.getState().agentTodos.get("a1")!;
      expect(todos).toHaveLength(1);
      expect(todos[0].id).toBe("t2");
    });

    it("setPartialMessage replaces previous value", () => {
      const store = useAgentStore.getState();
      store.setPartialMessage("a1", "partial");
      expect(useAgentStore.getState().agentPartialMessages.get("a1")).toBe("partial");

      store.setPartialMessage("a1", "updated partial");
      expect(useAgentStore.getState().agentPartialMessages.get("a1")).toBe("updated partial");
    });

    it("appendPermissionRequest creates and accumulates entries", () => {
      const store = useAgentStore.getState();
      store.appendPermissionRequest("a1", { requestId: "pr-1", toolName: "bash" });
      store.appendPermissionRequest("a1", { requestId: "pr-2", elicitationType: "confirm" });

      const reqs = useAgentStore.getState().agentPermissionRequests.get("a1")!;
      expect(reqs).toHaveLength(2);
      expect(reqs[0]).toEqual(expect.objectContaining({ requestId: "pr-1" }));
      expect(reqs[1]).toEqual(expect.objectContaining({ requestId: "pr-2" }));
    });

    it("clearAgentStreamData removes data for the target id but not others", () => {
      const store = useAgentStore.getState();

      store.appendToolCall("a1", { toolName: "read_file" });
      store.appendToolResult("a1", { toolName: "read_file" });
      store.setReasoning("a1", "thinking");
      store.appendFileChanges("a1", [{ path: "x.ts" }]);
      store.setTodos("a1", [{ id: "t1", content: "do it" }]);
      store.setPartialMessage("a1", "msg");
      store.appendPermissionRequest("a1", { requestId: "pr-1" });

      store.appendToolCall("a2", { toolName: "bash" });
      store.setReasoning("a2", "other");

      store.clearAgentStreamData("a1");

      const s = useAgentStore.getState();
      expect(s.agentToolCalls.has("a1")).toBe(false);
      expect(s.agentToolResults.has("a1")).toBe(false);
      expect(s.agentReasoning.has("a1")).toBe(false);
      expect(s.agentFileChanges.has("a1")).toBe(false);
      expect(s.agentTodos.has("a1")).toBe(false);
      expect(s.agentPartialMessages.has("a1")).toBe(false);
      expect(s.agentPermissionRequests.has("a1")).toBe(false);

      expect(s.agentToolCalls.get("a2")).toHaveLength(1);
      expect(s.agentReasoning.get("a2")).toBe("other");
    });
  });
});
