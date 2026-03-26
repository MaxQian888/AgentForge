jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useFormStore } from "./form-store";

describe("useFormStore", () => {
  beforeEach(() => {
    useFormStore.setState({
      formsByProject: {},
      formsBySlug: {},
      loadingByProject: {},
    });
  });

  it("fetches forms, fetches forms by slug, and submits public forms", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValueOnce({
      data: [{ id: "form-1", projectId: "project-1", name: "Bug", slug: "bug", fields: [], targetStatus: "inbox", isPublic: true, createdAt: "", updatedAt: "" }],
    }).mockResolvedValueOnce({
      data: { id: "form-1", projectId: "project-1", name: "Bug", slug: "bug", fields: [], targetStatus: "inbox", isPublic: true, createdAt: "", updatedAt: "" },
    });
    api.post.mockResolvedValueOnce({
      data: { id: "form-1", projectId: "project-1", name: "Bug", slug: "bug", fields: [], targetStatus: "inbox", isPublic: true, createdAt: "", updatedAt: "" },
    }).mockResolvedValueOnce({ data: { id: "task-1" } });

    await useFormStore.getState().fetchForms("project-1");
    const publicForm = await useFormStore.getState().fetchFormBySlug("bug");
    await useFormStore.getState().createForm("project-1", { name: "Bug", slug: "bug", fields: [], targetStatus: "inbox", isPublic: true });
    const task = await useFormStore.getState().submitForm("bug", { values: { title: "Broken" } });

    expect(useFormStore.getState().formsByProject["project-1"]).toHaveLength(2);
    expect(publicForm.slug).toBe("bug");
    expect(useFormStore.getState().formsBySlug["bug"]).toMatchObject({
      id: "form-1",
      slug: "bug",
    });
    expect(task.id).toBe("task-1");
  });
});
