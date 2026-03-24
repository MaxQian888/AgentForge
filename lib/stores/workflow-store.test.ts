import { useWorkflowStore } from "./workflow-store";

const mockGet = jest.fn();
const mockPut = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({ get: mockGet, put: mockPut }),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

beforeEach(() => {
  useWorkflowStore.setState({
    config: null,
    loading: false,
    saving: false,
    error: null,
  });
  mockGet.mockReset();
  mockPut.mockReset();
});

describe("useWorkflowStore", () => {
  it("fetches workflow config", async () => {
    const mockConfig = {
      id: "wf-1",
      projectId: "proj-1",
      transitions: { inbox: ["triaged"] },
      triggers: [{ fromStatus: "triaged", toStatus: "assigned", action: "auto_assign" }],
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
});
