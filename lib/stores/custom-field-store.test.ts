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

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

function makeApiClient() {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };
}

describe("useCustomFieldStore", () => {
  const api = makeApiClient();

  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
    api.put.mockReset();
    api.delete.mockReset();
    (createApiClient as jest.Mock).mockReturnValue(api);
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    useCustomFieldStore.setState({
      definitionsByProject: {},
      valuesByTask: {},
      loadingByProject: {},
      errorByProject: {},
    });
  });

  it("fetches definitions and updates task values", async () => {
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

  it("stores fetch failures and clears the project loading flag", async () => {
    api.get.mockRejectedValueOnce(new Error("boom"));

    await useCustomFieldStore.getState().fetchDefinitions("project-1");

    expect(useCustomFieldStore.getState()).toMatchObject({
      loadingByProject: { "project-1": false },
      errorByProject: { "project-1": "Unable to load custom fields" },
    });
  });

  it("creates, updates, deletes, and reorders field definitions", async () => {
    useCustomFieldStore.setState({
      definitionsByProject: {
        "project-1": [
          { id: "field-1", projectId: "project-1", name: "Priority", fieldType: "select", options: ["P0"], sortOrder: 1, required: true, createdAt: "", updatedAt: "" },
          { id: "field-2", projectId: "project-1", name: "Risk", fieldType: "text", options: [], sortOrder: 2, required: false, createdAt: "", updatedAt: "" },
        ],
      },
      valuesByTask: {},
      loadingByProject: {},
      errorByProject: {},
    });
    api.post.mockResolvedValueOnce({
      data: { id: "field-3", projectId: "project-1", name: "Owner", fieldType: "text", options: [], sortOrder: 3, required: false, createdAt: "", updatedAt: "" },
    });
    api.put
      .mockResolvedValueOnce({
        data: { id: "field-3", projectId: "project-1", name: "Owner Team", fieldType: "text", options: [], sortOrder: 3, required: true, createdAt: "", updatedAt: "" },
      })
      .mockResolvedValueOnce({ data: {} });
    api.delete.mockResolvedValueOnce({});

    await useCustomFieldStore.getState().createDefinition("project-1", {
      name: "Owner",
      fieldType: "text",
    });
    await useCustomFieldStore.getState().updateDefinition("project-1", "field-3", {
      name: "Owner Team",
      required: true,
    });
    await useCustomFieldStore.getState().reorderDefinitions("project-1", [
      "field-2",
      "field-3",
      "field-1",
    ]);
    await useCustomFieldStore.getState().deleteDefinition("project-1", "field-1");

    expect(useCustomFieldStore.getState().definitionsByProject["project-1"]).toEqual([
      expect.objectContaining({ id: "field-2", sortOrder: 1 }),
      expect.objectContaining({ id: "field-3", name: "Owner Team", sortOrder: 2 }),
    ]);
  });

  it("clears a task value after it was loaded", async () => {
    useCustomFieldStore.setState({
      definitionsByProject: {},
      valuesByTask: {
        "task-1": [
          { id: "value-1", taskId: "task-1", fieldDefId: "field-1", value: "P1", createdAt: "", updatedAt: "" },
        ],
      },
      loadingByProject: {},
      errorByProject: {},
    });
    api.delete.mockResolvedValueOnce({});

    await useCustomFieldStore.getState().clearTaskValue(
      "project-1",
      "task-1",
      "field-1",
    );

    expect(useCustomFieldStore.getState().valuesByTask["task-1"]).toEqual([]);
  });

  it("returns early without an access token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useCustomFieldStore.getState().fetchDefinitions("project-1");
    await useCustomFieldStore.getState().createDefinition("project-1", {
      name: "Skipped",
      fieldType: "text",
    });
    await useCustomFieldStore.getState().updateDefinition("project-1", "field-1", {
      name: "Skipped",
    });
    await useCustomFieldStore.getState().deleteDefinition("project-1", "field-1");
    await useCustomFieldStore.getState().reorderDefinitions("project-1", ["field-1"]);
    await useCustomFieldStore.getState().fetchTaskValues("project-1", "task-1");
    await useCustomFieldStore.getState().setTaskValue(
      "project-1",
      "task-1",
      "field-1",
      "P1",
    );
    await useCustomFieldStore.getState().clearTaskValue(
      "project-1",
      "task-1",
      "field-1",
    );

    expect(createApiClient).not.toHaveBeenCalled();
  });
});
