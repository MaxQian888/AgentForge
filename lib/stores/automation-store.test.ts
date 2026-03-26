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

describe("useAutomationStore", () => {
  beforeEach(() => {
    useAutomationStore.setState({
      rulesByProject: {},
      logsByProject: {},
    });
  });

  it("fetches rules and logs", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get
      .mockResolvedValueOnce({ data: [{ id: "rule-1", projectId: "project-1", name: "Done", enabled: true, eventType: "task.status_changed", conditions: [], actions: [], createdBy: "user-1", createdAt: "", updatedAt: "" }] })
      .mockResolvedValueOnce({ data: { items: [{ id: "log-1", ruleId: "rule-1", eventType: "task.status_changed", triggeredAt: "", status: "success", detail: {} }] } });

    await useAutomationStore.getState().fetchRules("project-1");
    await useAutomationStore.getState().fetchLogs("project-1");

    expect(useAutomationStore.getState().rulesByProject["project-1"]).toHaveLength(1);
    expect(useAutomationStore.getState().logsByProject["project-1"]).toHaveLength(1);
  });
});
