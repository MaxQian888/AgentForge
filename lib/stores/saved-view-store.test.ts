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

describe("useSavedViewStore", () => {
  beforeEach(() => {
    useSavedViewStore.setState({
      viewsByProject: {},
      currentViewByProject: {},
      loadingByProject: {},
    });
  });

  it("loads views and updates default selection", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: [
        { id: "view-1", projectId: "project-1", name: "Default", isDefault: true, sharedWith: {}, config: {}, createdAt: "", updatedAt: "" },
      ],
    });

    await useSavedViewStore.getState().fetchViews("project-1");
    expect(useSavedViewStore.getState().currentViewByProject["project-1"]).toBe("view-1");

    api.post.mockResolvedValue({});
    await useSavedViewStore.getState().setDefaultView("project-1", "view-1");
    expect(useSavedViewStore.getState().viewsByProject["project-1"][0].isDefault).toBe(true);
  });
});
