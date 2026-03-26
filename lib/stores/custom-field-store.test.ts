jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useCustomFieldStore } from "./custom-field-store";

function makeApiClient() {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };
}

describe("useCustomFieldStore", () => {
  beforeEach(() => {
    useCustomFieldStore.setState({
      definitionsByProject: {},
      valuesByTask: {},
      loadingByProject: {},
      errorByProject: {},
    });
  });

  it("fetches definitions and updates task values", async () => {
    const api = makeApiClient();
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get
      .mockResolvedValueOnce({
        data: [{ id: "field-1", projectId: "project-1", name: "Priority", fieldType: "select", options: ["P0"], sortOrder: 1, required: true, createdAt: "", updatedAt: "" }],
      })
      .mockResolvedValueOnce({
        data: [{ id: "value-1", taskId: "task-1", fieldDefId: "field-1", value: "P0", createdAt: "", updatedAt: "" }],
      });
    api.put.mockResolvedValue({
      data: { id: "value-1", taskId: "task-1", fieldDefId: "field-1", value: "P1", createdAt: "", updatedAt: "" },
    });

    await useCustomFieldStore.getState().fetchDefinitions("project-1");
    await useCustomFieldStore.getState().fetchTaskValues("project-1", "task-1");
    await useCustomFieldStore.getState().setTaskValue("project-1", "task-1", "field-1", "P1");

    expect(useCustomFieldStore.getState().definitionsByProject["project-1"]).toHaveLength(1);
    expect(useCustomFieldStore.getState().valuesByTask["task-1"][0].value).toBe("P1");
  });
});
