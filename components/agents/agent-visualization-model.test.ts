import type {
  Agent,
  AgentPoolSummary,
  BridgeHealthSummary,
} from "@/lib/stores/agent-store";
import type { CodingAgentCatalog } from "@/lib/stores/project-store";

type BuildAgentVisualizationModel = typeof import("./agent-visualization-model").buildAgentVisualizationModel;

let buildAgentVisualizationModel: BuildAgentVisualizationModel | undefined;

beforeAll(async () => {
  const mod = await import("./agent-visualization-model").catch(() => null);
  buildAgentVisualizationModel = mod?.buildAgentVisualizationModel;
});

describe("buildAgentVisualizationModel", () => {
  const agents: Agent[] = [
    {
      id: "agent-1",
      taskId: "task-1",
      taskTitle: "Implement graph mapping",
      memberId: "member-1",
      roleId: "coder",
      roleName: "Coder",
      status: "running",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 4,
      cost: 3.6,
      budget: 5,
      worktreePath: "",
      branchName: "agent/task-1",
      sessionId: "session-1",
      lastActivity: "2026-04-01T08:00:00.000Z",
      startedAt: "2026-04-01T07:30:00.000Z",
      createdAt: "2026-04-01T07:30:00.000Z",
      canResume: false,
      memoryStatus: "none",
      dispatchStatus: "started",
      guardrailType: "",
    },
    {
      id: "agent-2",
      taskId: "task-2",
      taskTitle: "Review runtime availability",
      memberId: "member-2",
      roleId: "reviewer",
      roleName: "Reviewer",
      status: "paused",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
      turns: 2,
      cost: 0.8,
      budget: 5,
      worktreePath: "",
      branchName: "agent/task-2",
      sessionId: "session-2",
      lastActivity: "2026-04-01T08:10:00.000Z",
      startedAt: "2026-04-01T08:00:00.000Z",
      createdAt: "2026-04-01T08:00:00.000Z",
      canResume: true,
      memoryStatus: "available",
      dispatchStatus: "blocked",
      guardrailType: "budget",
    },
    {
      id: "agent-3",
      taskId: "task-3",
      taskTitle: "Fallback runtime path",
      memberId: "member-3",
      roleId: "planner",
      roleName: "Planner",
      status: "starting",
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      turns: 1,
      cost: 0.2,
      budget: 5,
      worktreePath: "",
      branchName: "agent/task-3",
      sessionId: "session-3",
      lastActivity: "2026-04-01T08:20:00.000Z",
      startedAt: "2026-04-01T08:18:00.000Z",
      createdAt: "2026-04-01T08:18:00.000Z",
      canResume: false,
      memoryStatus: "warming",
      dispatchStatus: "queued",
      guardrailType: "pool",
    },
  ];

  const pool: AgentPoolSummary = {
    active: 2,
    max: 4,
    available: 2,
    pausedResumable: 1,
    queued: 2,
    warm: 1,
    degraded: true,
    queue: [
      {
        entryId: "queue-1",
        projectId: "project-1",
        taskId: "task-2",
        memberId: "member-2",
        runtime: "codex",
        provider: "openai",
        priority: 20,
        status: "blocked",
        reason: "budget",
        createdAt: "2026-04-01T08:05:00.000Z",
        updatedAt: "2026-04-01T08:05:00.000Z",
      },
      {
        entryId: "queue-2",
        projectId: "project-1",
        taskId: "task-4",
        memberId: "member-4",
        runtime: "codex",
        provider: "openai",
        priority: 10,
        status: "queued",
        reason: "agent pool is at capacity",
        createdAt: "2026-04-01T08:06:00.000Z",
        updatedAt: "2026-04-01T08:06:00.000Z",
      },
    ],
  };

  const runtimeCatalog: CodingAgentCatalog = {
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
        available: true,
        diagnostics: [],
        supportedFeatures: ["reasoning"],
      },
      {
        runtime: "claude_code",
        label: "Claude Code",
        defaultProvider: "anthropic",
        compatibleProviders: ["anthropic"],
        defaultModel: "claude-sonnet-4-5",
        modelOptions: ["claude-sonnet-4-5"],
        available: false,
        diagnostics: [
          { code: "missing_cli", message: "CLI missing", blocking: true },
        ],
        supportedFeatures: ["session_resume"],
      },
    ],
  };

  const bridgeHealth: BridgeHealthSummary = {
    status: "degraded",
    lastCheck: "2026-04-01T08:15:00.000Z",
    pool: {
      active: 2,
      available: 2,
      warm: 1,
    },
  };

  it("groups shared runtime targets and exposes degraded summary metadata", () => {
    expect(typeof buildAgentVisualizationModel).toBe("function");

    const model = buildAgentVisualizationModel!({
      agents,
      pool,
      runtimeCatalog,
      bridgeHealth,
      requestedMemberId: null,
    });

    expect(model.summary.isDegraded).toBe(true);
    expect(model.summary.agentCount).toBe(3);
    expect(model.summary.queueCount).toBe(2);
    expect(model.summary.runtimeCount).toBe(2);
    expect(
      model.nodes.filter((node) => node.id.startsWith("runtime:")),
    ).toHaveLength(2);
    expect(
      model.nodes.filter((node) => node.id.startsWith("agent:")),
    ).toHaveLength(3);
    expect(
      model.nodes.find((node) => node.id === "runtime:codex:openai:gpt-5.4"),
    ).toBeDefined();
  });

  it("scopes queue and agent relationships to the selected member", () => {
    expect(typeof buildAgentVisualizationModel).toBe("function");

    const model = buildAgentVisualizationModel!({
      agents,
      pool,
      runtimeCatalog,
      bridgeHealth,
      requestedMemberId: "member-2",
    });

    expect(model.summary.agentCount).toBe(1);
    expect(model.summary.queueCount).toBe(1);
    expect(
      model.nodes.find((node) => node.id === "agent:agent-2"),
    ).toBeDefined();
    expect(
      model.nodes.find((node) => node.id === "agent:agent-1"),
    ).toBeUndefined();
    expect(
      model.nodes.find((node) => node.id === "dispatch:queue-1"),
    ).toBeDefined();
    expect(
      model.nodes.find((node) => node.id === "dispatch:queue-2"),
    ).toBeUndefined();
  });

  it("emits stable focus metadata for task, dispatch, and runtime nodes", () => {
    expect(typeof buildAgentVisualizationModel).toBe("function");

    const model = buildAgentVisualizationModel!({
      agents,
      pool,
      runtimeCatalog,
      bridgeHealth,
      requestedMemberId: null,
    });

    expect(model.focusByNodeId["task:task-2"]).toMatchObject({
      kind: "task",
      nodeId: "task:task-2",
      taskId: "task-2",
      taskTitle: "Review runtime availability",
      agentCount: 1,
      queueCount: 1,
    });
    expect(model.focusByNodeId["dispatch:queue-1"]).toMatchObject({
      kind: "dispatch",
      nodeId: "dispatch:queue-1",
      entryId: "queue-1",
      taskId: "task-2",
      status: "blocked",
      priority: 20,
      reason: "budget",
    });
    expect(model.focusByNodeId["runtime:claude_code:anthropic:claude-sonnet-4-5"]).toMatchObject({
      kind: "runtime",
      nodeId: "runtime:claude_code:anthropic:claude-sonnet-4-5",
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      available: false,
      diagnostics: [
        expect.objectContaining({
          code: "missing_cli",
        }),
      ],
    });
  });
});
