jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { flattenDocsTree, findDocsPageById, useDocsStore } from "./docs-store";

describe("useDocsStore", () => {
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
    useDocsStore.setState({
      projectId: null,
      tree: [],
      currentPage: null,
      comments: [],
      versions: [],
      templates: [],
      favorites: [],
      recentAccess: [],
      loading: false,
      saving: false,
      error: null,
    });
  });

  it("hydrates tree and page workspace DTOs", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "page-1",
            spaceId: "space-1",
            title: "Docs",
            content: "[]",
            contentText: "",
            path: "/page-1",
            sortOrder: 0,
            isTemplate: false,
            isSystem: false,
            isPinned: true,
            createdAt: "2026-03-26T12:00:00.000Z",
            updatedAt: "2026-03-26T12:00:00.000Z",
            children: [
              {
                id: "page-2",
                spaceId: "space-1",
                parentId: "page-1",
                title: "Runbook",
                content: "[]",
                contentText: "",
                path: "/page-1/page-2",
                sortOrder: 0,
                isTemplate: false,
                isSystem: false,
                isPinned: false,
                createdAt: "2026-03-26T12:05:00.000Z",
                updatedAt: "2026-03-26T12:05:00.000Z",
                children: [],
              },
            ],
          },
        ])
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "page-2",
          spaceId: "space-1",
          title: "Runbook",
          content: '[{"type":"paragraph"}]',
          contentText: "Runbook",
          path: "/page-1/page-2",
          sortOrder: 0,
          isTemplate: false,
          isSystem: false,
          isPinned: false,
          createdAt: "2026-03-26T12:05:00.000Z",
          updatedAt: "2026-03-26T12:05:00.000Z",
        })
      )
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "comment-1",
            pageId: "page-2",
            body: "Looks good",
            mentions: "[]",
            createdAt: "2026-03-26T12:06:00.000Z",
            updatedAt: "2026-03-26T12:06:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "version-1",
            pageId: "page-2",
            versionNumber: 1,
            name: "Initial",
            content: "[]",
            createdAt: "2026-03-26T12:07:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useDocsStore.getState().fetchTree("project-1");
    await useDocsStore.getState().fetchPageWorkspace("project-1", "page-2");

    expect(useDocsStore.getState().tree).toHaveLength(1);
    expect(flattenDocsTree(useDocsStore.getState().tree)).toHaveLength(2);
    expect(findDocsPageById(useDocsStore.getState().tree, "page-2")).toEqual(
      expect.objectContaining({
        id: "page-2",
        title: "Runbook",
      })
    );
    expect(useDocsStore.getState().currentPage).toEqual(
      expect.objectContaining({
        id: "page-2",
        title: "Runbook",
      })
    );
    expect(useDocsStore.getState().comments[0]).toEqual(
      expect.objectContaining({
        id: "comment-1",
        body: "Looks good",
      })
    );
    expect(useDocsStore.getState().versions[0]).toEqual(
      expect.objectContaining({
        id: "version-1",
        versionNumber: 1,
      })
    );
  });

  it("creates a page and refreshes tree, and toggles favorites", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "page-3",
          spaceId: "space-1",
          title: "ADR",
          content: "[]",
          contentText: "",
          path: "/page-3",
          sortOrder: 1,
          isTemplate: false,
          isSystem: false,
          isPinned: false,
          createdAt: "2026-03-26T12:08:00.000Z",
          updatedAt: "2026-03-26T12:08:00.000Z",
        }, 201)
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse({}, 200))
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            pageId: "page-3",
            userId: "user-1",
            createdAt: "2026-03-26T12:09:00.000Z",
          },
        ])
      );

    const page = await useDocsStore.getState().createPage({
      projectId: "project-1",
      title: "ADR",
    });
    await useDocsStore.getState().toggleFavorite({
      projectId: "project-1",
      pageId: "page-3",
      favorite: true,
    });

    expect(page).toEqual(expect.objectContaining({ id: "page-3", title: "ADR" }));
    expect(useDocsStore.getState().favorites).toEqual([
      expect.objectContaining({ pageId: "page-3", userId: "user-1" }),
    ]);
  });

  it("resolves page context by page id so deep links can bootstrap project state", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        projectId: "project-2",
        page: {
          id: "page-9",
          spaceId: "space-9",
          title: "Shared page",
          content: "[]",
          contentText: "",
          path: "/shared-page",
          sortOrder: 0,
          isTemplate: false,
          isSystem: false,
          isPinned: false,
          createdAt: "2026-03-26T12:10:00.000Z",
          updatedAt: "2026-03-26T12:10:00.000Z",
        },
      }),
    );

    const resolvedProjectId = await useDocsStore.getState().resolvePageContext("page-9");

    expect(resolvedProjectId).toBe("project-2");
    expect(useDocsStore.getState()).toMatchObject({
      projectId: "project-2",
      currentPage: expect.objectContaining({
        id: "page-9",
        title: "Shared page",
      }),
    });
  });

  it("supports template filters and template CRUD helpers", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "template-1",
            spaceId: "space-1",
            title: "Runbook",
            content: "[]",
            contentText: "Operational checklist",
            path: "/templates/runbook/template-1",
            sortOrder: 0,
            isTemplate: true,
            templateCategory: "runbook",
            isSystem: true,
            isPinned: false,
            templateSource: "system",
            previewSnippet: "Operational checklist",
            canEdit: false,
            canDelete: false,
            canDuplicate: true,
            canUse: true,
            createdAt: "2026-03-26T12:00:00.000Z",
            updatedAt: "2026-03-26T12:00:00.000Z",
          },
        ]),
      )
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "template-2",
            spaceId: "space-1",
            title: "Custom Template",
            content: "[]",
            contentText: "Steps",
            path: "/templates/custom/template-2",
            sortOrder: 0,
            isTemplate: true,
            templateCategory: "custom",
            isSystem: false,
            isPinned: false,
            createdAt: "2026-03-26T12:11:00.000Z",
            updatedAt: "2026-03-26T12:11:00.000Z",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "template-2",
            spaceId: "space-1",
            title: "Custom Template",
            content: '[{"type":"paragraph","content":"updated"}]',
            contentText: "updated",
            path: "/templates/custom/template-2",
            sortOrder: 0,
            isTemplate: true,
            templateCategory: "runbook",
            isSystem: false,
            isPinned: false,
            createdAt: "2026-03-26T12:11:00.000Z",
            updatedAt: "2026-03-26T12:12:00.000Z",
          },
        ),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "template-3",
            spaceId: "space-1",
            title: "Runbook Copy",
            content: "[]",
            contentText: "copy",
            path: "/templates/custom/template-3",
            sortOrder: 0,
            isTemplate: true,
            templateCategory: "runbook",
            isSystem: false,
            isPinned: false,
            createdAt: "2026-03-26T12:13:00.000Z",
            updatedAt: "2026-03-26T12:13:00.000Z",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse({}, 200))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useDocsStore.getState().fetchTemplates("project-1", {
      query: "runbook",
      category: "runbook",
      source: "system",
    });
    const created = await useDocsStore.getState().createTemplate({
      projectId: "project-1",
      title: "Custom Template",
      category: "custom",
    });
    const updated = await useDocsStore.getState().updateTemplate({
      projectId: "project-1",
      templateId: "template-2",
      title: "Custom Template",
      content: '[{"type":"paragraph","content":"updated"}]',
      contentText: "updated",
      templateCategory: "runbook",
    });
    const duplicated = await useDocsStore.getState().duplicateTemplate({
      projectId: "project-1",
      templateId: "template-1",
      name: "Runbook Copy",
      category: "runbook",
    });
    await useDocsStore.getState().deleteTemplate({
      projectId: "project-1",
      templateId: "template-2",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "http://localhost:7777/api/v1/projects/project-1/wiki/templates?q=runbook&category=runbook&source=system",
      expect.any(Object),
    );
    expect(created).toEqual(expect.objectContaining({ id: "template-2", isTemplate: true }));
    expect(updated).toEqual(
      expect.objectContaining({
        id: "template-2",
        templateCategory: "runbook",
      }),
    );
    expect(duplicated).toEqual(expect.objectContaining({ id: "template-3", title: "Runbook Copy" }));
    expect(useDocsStore.getState().templates).toEqual([]);
  });
});
