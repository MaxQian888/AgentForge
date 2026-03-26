jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useEntityLinkStore } from "./entity-link-store";

describe("useEntityLinkStore", () => {
  const fetchMock = jest.fn();
  const mockJsonResponse = (data: unknown, status = 200) =>
    ({
      ok: status >= 200 && status < 300,
      status,
      json: async () => data,
    }) as Response;

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useEntityLinkStore.setState({
      linksByEntity: {},
      loading: false,
      error: null,
    });
  });

  it("lists links for an entity and creates/deletes links", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "link-1",
            projectId: "project-1",
            sourceType: "task",
            sourceId: "task-1",
            targetType: "wiki_page",
            targetId: "page-1",
            linkType: "requirement",
            createdBy: "user-1",
            createdAt: "2026-03-26T10:00:00.000Z",
          },
        ]),
      )
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "link-2",
            projectId: "project-1",
            sourceType: "task",
            sourceId: "task-1",
            targetType: "wiki_page",
            targetId: "page-2",
            linkType: "design",
            createdBy: "user-1",
            createdAt: "2026-03-26T10:01:00.000Z",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(mockJsonResponse({}))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useEntityLinkStore.getState().fetchLinks("project-1", "task", "task-1");
    expect(useEntityLinkStore.getState().linksByEntity["task:task-1"]).toHaveLength(1);

    await useEntityLinkStore.getState().createLink({
      projectId: "project-1",
      sourceType: "task",
      sourceId: "task-1",
      targetType: "wiki_page",
      targetId: "page-2",
      linkType: "design",
    });
    expect(useEntityLinkStore.getState().linksByEntity["task:task-1"]).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: "link-2", linkType: "design" })]),
    );

    await useEntityLinkStore.getState().deleteLink("project-1", "task", "task-1", "link-2");
    expect(useEntityLinkStore.getState().linksByEntity["task:task-1"]).toEqual([]);
  });
});
