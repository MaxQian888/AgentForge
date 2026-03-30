jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useSavedViewStore } from "./saved-view-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

const createApiClientMock = createApiClient as jest.MockedFunction<
  typeof createApiClient
>;

function createView(id: string, overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id,
    projectId: "project-1",
    name: `View ${id}`,
    isDefault: false,
    sharedWith: {},
    config: {},
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

describe("useSavedViewStore", () => {
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
    useSavedViewStore.setState({
      viewsByProject: {},
      currentViewByProject: {},
      loadingByProject: {},
    });
  });

  it("loads views and updates default selection", async () => {
    api.get.mockResolvedValue({
      data: [
        createView("view-1", { name: "Default", isDefault: true }),
      ],
    });

    await useSavedViewStore.getState().fetchViews("project-1");
    expect(useSavedViewStore.getState().currentViewByProject["project-1"]).toBe("view-1");

    api.post.mockResolvedValue({});
    await useSavedViewStore.getState().setDefaultView("project-1", "view-1");
    expect(useSavedViewStore.getState().viewsByProject["project-1"][0].isDefault).toBe(true);
  });

  it("preserves an existing selection when views are reloaded", async () => {
    useSavedViewStore.setState({
      viewsByProject: {},
      currentViewByProject: { "project-1": "view-existing" },
      loadingByProject: {},
    });
    api.get.mockResolvedValue({
      data: [
        createView("view-1", { isDefault: true }),
        createView("view-existing"),
      ],
    });

    await useSavedViewStore.getState().fetchViews("project-1");

    expect(useSavedViewStore.getState()).toMatchObject({
      currentViewByProject: { "project-1": "view-existing" },
      loadingByProject: { "project-1": false },
    });
  });

  it("allows manually selecting a saved view", () => {
    useSavedViewStore.getState().selectView("project-1", "view-2");

    expect(useSavedViewStore.getState().currentViewByProject["project-1"]).toBe(
      "view-2",
    );
  });

  it("creates and updates views for a project", async () => {
    api.post.mockResolvedValueOnce({
      data: createView("view-2", { name: "Release board" }),
    });
    api.put.mockResolvedValueOnce({
      data: createView("view-2", { name: "Updated board" }),
    });

    await useSavedViewStore.getState().createView("project-1", {
      name: "Release board",
      config: { layout: "board" },
    });
    await useSavedViewStore.getState().updateView("project-1", "view-2", {
      name: "Updated board",
    });

    expect(useSavedViewStore.getState()).toMatchObject({
      viewsByProject: {
        "project-1": [expect.objectContaining({ id: "view-2", name: "Updated board" })],
      },
      currentViewByProject: {
        "project-1": "view-2",
      },
    });
  });

  it("deletes views and falls back to another available selection", async () => {
    useSavedViewStore.setState({
      viewsByProject: {
        "project-1": [createView("view-1"), createView("view-2")],
      },
      currentViewByProject: {
        "project-1": "view-1",
      },
      loadingByProject: {},
    });
    api.delete.mockResolvedValue({});

    await useSavedViewStore.getState().deleteView("project-1", "view-1");

    expect(useSavedViewStore.getState()).toMatchObject({
      viewsByProject: {
        "project-1": [expect.objectContaining({ id: "view-2" })],
      },
      currentViewByProject: {
        "project-1": "view-2",
      },
    });
  });

  it("marks the requested view as default and selects it", async () => {
    useSavedViewStore.setState({
      viewsByProject: {
        "project-1": [
          createView("view-1", { isDefault: true }),
          createView("view-2"),
        ],
      },
      currentViewByProject: {},
      loadingByProject: {},
    });
    api.post.mockResolvedValue({});

    await useSavedViewStore.getState().setDefaultView("project-1", "view-2");

    expect(useSavedViewStore.getState().viewsByProject["project-1"]).toEqual([
      expect.objectContaining({ id: "view-1", isDefault: false }),
      expect.objectContaining({ id: "view-2", isDefault: true }),
    ]);
    expect(useSavedViewStore.getState().currentViewByProject["project-1"]).toBe(
      "view-2",
    );
  });

  it("returns early without an access token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({ accessToken: null });

    await useSavedViewStore.getState().fetchViews("project-1");
    await useSavedViewStore.getState().createView("project-1", {
      name: "Hidden",
      config: {},
    });
    await useSavedViewStore.getState().updateView("project-1", "view-1", {
      name: "Updated",
    });
    await useSavedViewStore.getState().deleteView("project-1", "view-1");
    await useSavedViewStore.getState().setDefaultView("project-1", "view-1");

    expect(createApiClientMock).not.toHaveBeenCalled();
  });
});
