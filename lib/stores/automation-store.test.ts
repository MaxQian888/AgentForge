jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useAutomationStore } from "./automation-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

const createApiClientMock = createApiClient as jest.MockedFunction<
  typeof createApiClient
>;

function createRule(id: string, overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id,
    projectId: "project-1",
    name: `Rule ${id}`,
    enabled: true,
    eventType: "task.status_changed",
    conditions: [],
    actions: [],
    createdBy: "user-1",
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

describe("useAutomationStore", () => {
  const api = {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };

  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
    api.put.mockReset();
    api.delete.mockReset();
    createApiClientMock.mockReturnValue(
      api as unknown as ReturnType<typeof createApiClient>,
    );
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    useAutomationStore.setState({
      rulesByProject: {},
      logsByProject: {},
    });
  });

  it("fetches rules and logs", async () => {
    api.get
      .mockResolvedValueOnce({ data: [createRule("rule-1", { name: "Done" })] })
      .mockResolvedValueOnce({
        data: {
          items: [
            {
              id: "log-1",
              ruleId: "rule-1",
              eventType: "task.status_changed",
              triggeredAt: "",
              status: "success",
              detail: {},
            },
          ],
        },
      });

    await useAutomationStore.getState().fetchRules("project-1");
    await useAutomationStore.getState().fetchLogs("project-1");

    expect(useAutomationStore.getState().rulesByProject["project-1"]).toHaveLength(1);
    expect(useAutomationStore.getState().logsByProject["project-1"]).toHaveLength(1);
  });

  it("creates, updates, and deletes rules in project state", async () => {
    useAutomationStore.setState({
      rulesByProject: {
        "project-1": [createRule("rule-1")],
      },
      logsByProject: {},
    });
    api.post.mockResolvedValueOnce({
      data: createRule("rule-2", { name: "Auto assign" }),
    });
    api.put.mockResolvedValueOnce({
      data: createRule("rule-2", { name: "Auto review", enabled: false }),
    });
    api.delete.mockResolvedValue({});

    await useAutomationStore.getState().createRule("project-1", {
      name: "Auto assign",
      enabled: true,
      eventType: "task.created",
      conditions: [],
      actions: [],
    });
    await useAutomationStore.getState().updateRule("project-1", "rule-2", {
      name: "Auto review",
      enabled: false,
    });
    await useAutomationStore.getState().deleteRule("project-1", "rule-1");

    expect(useAutomationStore.getState().rulesByProject["project-1"]).toEqual([
      expect.objectContaining({
        id: "rule-2",
        name: "Auto review",
        enabled: false,
      }),
    ]);
  });

  it("falls back to an empty log list when the API payload omits items", async () => {
    api.get.mockResolvedValueOnce({ data: { items: undefined } });

    await useAutomationStore.getState().fetchLogs("project-1");

    expect(useAutomationStore.getState().logsByProject["project-1"]).toEqual([]);
  });

  it("returns early without an access token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({ accessToken: null });

    await useAutomationStore.getState().fetchRules("project-1");
    await useAutomationStore.getState().createRule("project-1", {
      name: "Skipped",
      enabled: true,
      eventType: "task.created",
      conditions: [],
      actions: [],
    });
    await useAutomationStore.getState().updateRule("project-1", "rule-1", {
      name: "Updated",
    });
    await useAutomationStore.getState().deleteRule("project-1", "rule-1");
    await useAutomationStore.getState().fetchLogs("project-1");

    expect(createApiClientMock).not.toHaveBeenCalled();
  });
});
