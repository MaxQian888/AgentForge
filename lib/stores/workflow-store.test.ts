import { useWorkflowStore } from "./workflow-store";

const mockGet = jest.fn();
const mockPut = jest.fn();
const mockPost = jest.fn();
const mockDelete = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({ get: mockGet, put: mockPut, post: mockPost, delete: mockDelete }),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: { getState: jest.fn(() => ({ accessToken: "test-token" })) },
}));

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

beforeEach(() => {
  useWorkflowStore.setState({
    config: null,
    loading: false,
    saving: false,
    error: null,
    recentActivityByProject: {},
    definitions: [],
    templates: [],
    selectedDefinition: null,
  });
  mockGet.mockReset();
  mockPut.mockReset();
  mockPost.mockReset();
  mockDelete.mockReset();
  authStoreModule.useAuthStore.getState.mockReturnValue({
    accessToken: "test-token",
  });
});

describe("useWorkflowStore", () => {
  it("fetches workflow config", async () => {
    const mockConfig = {
      id: "wf-1",
      projectId: "proj-1",
      transitions: { inbox: ["triaged"] },
      triggers: [{ fromStatus: "triaged", toStatus: "assigned", action: "dispatch_agent" }],
    };

    mockGet.mockResolvedValueOnce({ data: mockConfig });

    await useWorkflowStore.getState().fetchWorkflow("proj-1");

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/workflow",
      { token: "test-token" }
    );
    expect(useWorkflowStore.getState().config).toEqual(mockConfig);
    expect(useWorkflowStore.getState().loading).toBe(false);
  });

  it("updates workflow config", async () => {
    const updated = {
      id: "wf-1",
      projectId: "proj-1",
      transitions: { inbox: ["triaged", "assigned"] },
      triggers: [],
    };

    mockPut.mockResolvedValueOnce({ data: updated });

    await useWorkflowStore.getState().updateWorkflow("proj-1", {
      transitions: { inbox: ["triaged", "assigned"] },
      triggers: [],
    });

    expect(mockPut).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/workflow",
      { transitions: { inbox: ["triaged", "assigned"] }, triggers: [] },
      { token: "test-token" }
    );
    expect(useWorkflowStore.getState().config).toEqual(updated);
    expect(useWorkflowStore.getState().saving).toBe(false);
  });

  it("sets error on fetch failure", async () => {
    mockGet.mockRejectedValueOnce(new Error("Network error"));

    await useWorkflowStore.getState().fetchWorkflow("proj-1");

    expect(useWorkflowStore.getState().error).toBe("Unable to load workflow config");
  });

  it("sets error on update failure", async () => {
    mockPut.mockRejectedValueOnce(new Error("Network error"));

    await useWorkflowStore.getState().updateWorkflow("proj-1", {
      transitions: {},
      triggers: [],
    });

    expect(useWorkflowStore.getState().error).toBe("Unable to save workflow config");
  });

  it("stores bounded recent workflow activity per project", () => {
    for (let index = 0; index < 12; index += 1) {
      useWorkflowStore.getState().appendActivity("proj-1", {
        taskId: `task-${index}`,
        action: "notify",
        from: "triaged",
        to: "assigned",
        timestamp: `2026-03-24T12:${String(index).padStart(2, "0")}:00.000Z`,
      });
    }

    const activity = useWorkflowStore.getState().recentActivityByProject["proj-1"];

    expect(activity).toHaveLength(10);
    expect(activity[0]).toEqual(
      expect.objectContaining({
        taskId: "task-11",
      }),
    );
    expect(activity.at(-1)).toEqual(
      expect.objectContaining({
        taskId: "task-2",
      }),
    );
  });

  it("uses a generated timestamp and can clear workflow activity", () => {
    jest.useFakeTimers().setSystemTime(new Date("2026-03-30T12:00:00.000Z"));

    useWorkflowStore.getState().appendActivity("proj-1", {
      taskId: "task-1",
      action: "notify",
      from: "triaged",
      to: "assigned",
    });
    expect(useWorkflowStore.getState().recentActivityByProject["proj-1"][0]).toEqual(
      expect.objectContaining({
        timestamp: "2026-03-30T12:00:00.000Z",
      }),
    );

    useWorkflowStore.getState().clearActivity("proj-1");
    expect(useWorkflowStore.getState().recentActivityByProject["proj-1"]).toEqual(
      [],
    );

    jest.useRealTimers();
  });

  it("returns early without an auth token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useWorkflowStore.getState().fetchWorkflow("proj-1");
    await expect(
      useWorkflowStore.getState().updateWorkflow("proj-1", {
        transitions: {},
        triggers: [],
      }),
    ).resolves.toBe(false);

    expect(mockGet).not.toHaveBeenCalled();
    expect(mockPut).not.toHaveBeenCalled();
  });

  it("fetchPendingReviews is a function on the store", () => {
    expect(typeof useWorkflowStore.getState().fetchPendingReviews).toBe("function");
  });

  it("fetches project-aware workflow templates", async () => {
    mockGet.mockResolvedValueOnce({
      data: [
        {
          id: "template-1",
          projectId: "00000000-0000-0000-0000-000000000000",
          name: "Plan Code Review",
          description: "System template",
          status: "template",
          category: "system",
          templateSource: "system",
          canClone: true,
          canExecute: true,
          nodes: [],
          edges: [],
          version: 1,
          createdAt: "2026-04-15T00:00:00.000Z",
          updatedAt: "2026-04-15T00:00:00.000Z",
        },
      ],
    });

    await useWorkflowStore.getState().fetchTemplates("proj-1", {
      query: "plan",
      source: "system",
    });

    expect(mockGet).toHaveBeenCalledWith(
      "/api/v1/workflow-templates?q=plan&source=system",
      { token: "test-token", headers: { "X-Project-ID": "proj-1" } },
    );
    expect(useWorkflowStore.getState().templates).toEqual([
      expect.objectContaining({ id: "template-1", templateSource: "system" }),
    ]);
  });

  it("publishes, duplicates, and deletes workflow templates", async () => {
    useWorkflowStore.setState({
      definitions: [
        {
          id: "workflow-9",
          projectId: "proj-1",
          name: "Active Workflow",
          description: "Project workflow",
          status: "active",
          category: "user",
          nodes: [],
          edges: [],
          version: 1,
          createdAt: "2026-04-15T00:00:00.000Z",
          updatedAt: "2026-04-15T00:00:00.000Z",
        },
      ],
    });

    mockPost
      .mockResolvedValueOnce({
        data: {
          id: "template-2",
          projectId: "proj-1",
          name: "Project Template",
          description: "Reusable flow",
          status: "template",
          category: "user",
          templateSource: "user",
          canEdit: true,
          canDelete: true,
          nodes: [],
          edges: [],
          version: 1,
          createdAt: "2026-04-15T00:00:00.000Z",
          updatedAt: "2026-04-15T00:00:00.000Z",
        },
      })
      .mockResolvedValueOnce({
        data: {
          id: "template-3",
          projectId: "proj-1",
          name: "Template Copy",
          description: "Custom copy",
          status: "template",
          category: "user",
          templateSource: "user",
          canEdit: true,
          canDelete: true,
          nodes: [],
          edges: [],
          version: 1,
          createdAt: "2026-04-15T00:01:00.000Z",
          updatedAt: "2026-04-15T00:01:00.000Z",
        },
      });
    mockDelete.mockResolvedValueOnce({});

    const published = await useWorkflowStore.getState().publishTemplate("wf-1", "proj-1", {
      name: "Project Template",
      description: "Reusable flow",
    });
    const duplicated = await useWorkflowStore.getState().duplicateTemplate("template-1", "proj-1", {
      name: "Template Copy",
      description: "Custom copy",
    });
    const deleted = await useWorkflowStore.getState().deleteTemplate("template-2", "proj-1");

    expect(mockPost).toHaveBeenNthCalledWith(
      1,
      "/api/v1/workflows/wf-1/publish-template",
      { name: "Project Template", description: "Reusable flow" },
      { token: "test-token", headers: { "X-Project-ID": "proj-1" } },
    );
    expect(mockPost).toHaveBeenNthCalledWith(
      2,
      "/api/v1/workflow-templates/template-1/duplicate",
      { name: "Template Copy", description: "Custom copy" },
      { token: "test-token", headers: { "X-Project-ID": "proj-1" } },
    );
    expect(mockDelete).toHaveBeenCalledWith("/api/v1/workflow-templates/template-2", {
      token: "test-token",
      headers: { "X-Project-ID": "proj-1" },
    });
    expect(published).toEqual(expect.objectContaining({ id: "template-2" }));
    expect(duplicated).toEqual(expect.objectContaining({ id: "template-3" }));
    expect(deleted).toBe(true);
    expect(useWorkflowStore.getState().templates).toEqual([
      expect.objectContaining({ id: "template-3" }),
    ]);
    expect(useWorkflowStore.getState().definitions).toEqual([
      expect.objectContaining({ id: "workflow-9", status: "active" }),
    ]);
  });
});
