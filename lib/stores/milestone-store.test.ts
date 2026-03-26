jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useMilestoneStore } from "./milestone-store";

describe("useMilestoneStore", () => {
  beforeEach(() => {
    useMilestoneStore.setState({
      milestonesByProject: {},
    });
  });

  it("fetches and updates milestones", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: [{ id: "mile-1", projectId: "project-1", name: "v1", status: "planned", description: "", createdAt: "", updatedAt: "" }],
    });
    api.put.mockResolvedValue({
      data: { id: "mile-1", projectId: "project-1", name: "v1", status: "in_progress", description: "", createdAt: "", updatedAt: "" },
    });

    await useMilestoneStore.getState().fetchMilestones("project-1");
    await useMilestoneStore.getState().updateMilestone("project-1", "mile-1", { status: "in_progress" });

    expect(useMilestoneStore.getState().milestonesByProject["project-1"][0].status).toBe("in_progress");
  });
});
