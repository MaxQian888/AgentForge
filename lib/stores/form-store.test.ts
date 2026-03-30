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

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

describe("useFormStore", () => {
  const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };

  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
    api.put.mockReset();
    api.delete.mockReset();
    (createApiClient as jest.Mock).mockReturnValue(api);
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
    useFormStore.setState({
      formsByProject: {},
      formsBySlug: {},
      loadingByProject: {},
    });
  });

  it("fetches forms, fetches forms by slug, and submits public forms", async () => {
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

  it("updates and deletes forms while keeping slug lookups in sync", async () => {
    useFormStore.setState({
      formsByProject: {
        "project-1": [
          {
            id: "form-1",
            projectId: "project-1",
            name: "Bug",
            slug: "bug",
            fields: [],
            targetStatus: "inbox",
            isPublic: true,
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      formsBySlug: {
        bug: {
          id: "form-1",
          projectId: "project-1",
          name: "Bug",
          slug: "bug",
          fields: [],
          targetStatus: "inbox",
          isPublic: true,
          createdAt: "",
          updatedAt: "",
        },
      },
      loadingByProject: {},
    });
    api.put.mockResolvedValueOnce({
      data: {
        id: "form-1",
        projectId: "project-1",
        name: "Bug Updated",
        slug: "bug-updated",
        fields: [],
        targetStatus: "triaged",
        isPublic: true,
        createdAt: "",
        updatedAt: "",
      },
    });
    api.delete.mockResolvedValueOnce({ data: {} });

    await useFormStore.getState().updateForm("project-1", "form-1", {
      name: "Bug Updated",
      slug: "bug-updated",
    });
    await useFormStore.getState().deleteForm("project-1", "form-1");

    expect(useFormStore.getState()).toMatchObject({
      formsByProject: { "project-1": [] },
    });
    expect(useFormStore.getState().formsBySlug).toEqual({});
  });

  it("returns early without a token for project-scoped mutations", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useFormStore.getState().fetchForms("project-1");
    await useFormStore.getState().createForm("project-1", {
      name: "Bug",
      slug: "bug",
      fields: [],
      targetStatus: "inbox",
      isPublic: true,
    });
    await useFormStore.getState().updateForm("project-1", "form-1", {
      name: "Bug Updated",
    });
    await useFormStore.getState().deleteForm("project-1", "form-1");

    expect(createApiClient).not.toHaveBeenCalled();
  });
});
