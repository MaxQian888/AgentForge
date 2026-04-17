jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import {
  flattenKnowledgeTree,
  findKnowledgeAssetById,
  useKnowledgeStore,
  selectWikiPageTree,
  selectIngestedFiles,
  selectTemplatesByCategory,
  type KnowledgeAsset,
  type KnowledgeAssetTreeNode,
} from "./knowledge-store";

describe("useKnowledgeStore", () => {
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
    useKnowledgeStore.setState({
      projectId: null,
      tree: [],
      ingestedFiles: [],
      templates: [],
      currentAsset: null,
      comments: [],
      versions: [],
      favorites: [],
      recentAccess: [],
      searchResults: null,
      loading: false,
      saving: false,
      uploading: false,
      error: null,
    });
  });

  it("hydrates tree and page workspace DTOs", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "page-1",
            projectId: "project-1",
            kind: "wiki_page",
            spaceId: "space-1",
            title: "Docs",
            contentJson: "[]",
            contentText: "",
            path: "/page-1",
            sortOrder: 0,
            isPinned: true,
            version: 1,
            createdAt: "2026-03-26T12:00:00.000Z",
            updatedAt: "2026-03-26T12:00:00.000Z",
            children: [
              {
                id: "page-2",
                projectId: "project-1",
                kind: "wiki_page",
                spaceId: "space-1",
                parentId: "page-1",
                title: "Runbook",
                contentJson: "[]",
                contentText: "",
                path: "/page-1/page-2",
                sortOrder: 0,
                isPinned: false,
                version: 1,
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
          projectId: "project-1",
          kind: "wiki_page",
          spaceId: "space-1",
          title: "Runbook",
          contentJson: '[{"type":"paragraph"}]',
          contentText: "Runbook",
          path: "/page-1/page-2",
          sortOrder: 0,
          isPinned: false,
          version: 1,
          createdAt: "2026-03-26T12:05:00.000Z",
          updatedAt: "2026-03-26T12:05:00.000Z",
        })
      )
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "comment-1",
            assetId: "page-2",
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
            assetId: "page-2",
            versionNumber: 1,
            name: "Initial",
            kindSnapshot: "wiki_page",
            contentJson: "[]",
            createdAt: "2026-03-26T12:07:00.000Z",
          },
        ])
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useKnowledgeStore.getState().fetchTree("project-1");
    await useKnowledgeStore.getState().fetchPageWorkspace("project-1", "page-2");

    expect(useKnowledgeStore.getState().tree).toHaveLength(1);
    expect(flattenKnowledgeTree(useKnowledgeStore.getState().tree)).toHaveLength(2);
    expect(findKnowledgeAssetById(useKnowledgeStore.getState().tree, "page-2")).toEqual(
      expect.objectContaining({
        id: "page-2",
        title: "Runbook",
      })
    );
    expect(useKnowledgeStore.getState().currentAsset).toEqual(
      expect.objectContaining({
        id: "page-2",
        title: "Runbook",
      })
    );
    expect(useKnowledgeStore.getState().comments[0]).toEqual(
      expect.objectContaining({
        id: "comment-1",
        body: "Looks good",
      })
    );
    expect(useKnowledgeStore.getState().versions[0]).toEqual(
      expect.objectContaining({
        id: "version-1",
        versionNumber: 1,
      })
    );
  });

  it("creates a page and refreshes tree, and toggles favorites", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "page-3",
            projectId: "project-1",
            kind: "wiki_page",
            spaceId: "space-1",
            title: "ADR",
            contentJson: "[]",
            contentText: "",
            path: "/page-3",
            sortOrder: 1,
            isPinned: false,
            version: 1,
            createdAt: "2026-03-26T12:08:00.000Z",
            updatedAt: "2026-03-26T12:08:00.000Z",
          },
          201,
        )
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse({}, 200))
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            assetId: "page-3",
            userId: "user-1",
            createdAt: "2026-03-26T12:09:00.000Z",
          },
        ])
      );

    const page = await useKnowledgeStore.getState().createPage({
      projectId: "project-1",
      title: "ADR",
    });
    await useKnowledgeStore.getState().toggleFavorite({
      projectId: "project-1",
      pageId: "page-3",
      favorite: true,
    });

    expect(page).toEqual(expect.objectContaining({ id: "page-3", title: "ADR" }));
    expect(useKnowledgeStore.getState().favorites).toEqual([
      expect.objectContaining({ assetId: "page-3", userId: "user-1" }),
    ]);
  });

  it("resolves page context by page id so deep links can bootstrap project state", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        projectId: "project-2",
        page: {
          id: "page-9",
          projectId: "project-2",
          kind: "wiki_page",
          spaceId: "space-9",
          title: "Shared page",
          contentJson: "[]",
          contentText: "",
          path: "/shared-page",
          sortOrder: 0,
          isPinned: false,
          version: 1,
          createdAt: "2026-03-26T12:10:00.000Z",
          updatedAt: "2026-03-26T12:10:00.000Z",
        },
      }),
    );

    const resolvedProjectId = await useKnowledgeStore.getState().resolvePageContext("page-9");

    expect(resolvedProjectId).toBe("project-2");
    expect(useKnowledgeStore.getState()).toMatchObject({
      projectId: "project-2",
      currentAsset: expect.objectContaining({
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
            projectId: "project-1",
            kind: "template",
            spaceId: "space-1",
            title: "Runbook",
            contentJson: "[]",
            contentText: "Operational checklist",
            path: "/templates/runbook/template-1",
            sortOrder: 0,
            templateCategory: "runbook",
            isSystemTemplate: true,
            isPinned: false,
            version: 1,
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
            projectId: "project-1",
            kind: "template",
            spaceId: "space-1",
            title: "Custom Template",
            contentJson: "[]",
            contentText: "Steps",
            path: "/templates/custom/template-2",
            sortOrder: 0,
            templateCategory: "custom",
            isSystemTemplate: false,
            isPinned: false,
            version: 1,
            createdAt: "2026-03-26T12:11:00.000Z",
            updatedAt: "2026-03-26T12:11:00.000Z",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "template-2",
          projectId: "project-1",
          kind: "template",
          spaceId: "space-1",
          title: "Custom Template",
          contentJson: '[{"type":"paragraph","content":"updated"}]',
          contentText: "updated",
          path: "/templates/custom/template-2",
          sortOrder: 0,
          templateCategory: "runbook",
          isSystemTemplate: false,
          isPinned: false,
          version: 2,
          createdAt: "2026-03-26T12:11:00.000Z",
          updatedAt: "2026-03-26T12:12:00.000Z",
        }),
      )
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(mockJsonResponse([]))
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "template-3",
            projectId: "project-1",
            kind: "template",
            spaceId: "space-1",
            title: "Runbook Copy",
            contentJson: "[]",
            contentText: "copy",
            path: "/templates/custom/template-3",
            sortOrder: 0,
            templateCategory: "runbook",
            isSystemTemplate: false,
            isPinned: false,
            version: 1,
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

    await useKnowledgeStore.getState().fetchTemplates("project-1", {
      query: "runbook",
      category: "runbook",
      source: "system",
    });
    const created = await useKnowledgeStore.getState().createTemplate({
      projectId: "project-1",
      title: "Custom Template",
      category: "custom",
    });
    const updated = await useKnowledgeStore.getState().updateTemplate({
      projectId: "project-1",
      templateId: "template-2",
      title: "Custom Template",
      content: '[{"type":"paragraph","content":"updated"}]',
      contentText: "updated",
      templateCategory: "runbook",
    });
    const duplicated = await useKnowledgeStore.getState().duplicateTemplate({
      projectId: "project-1",
      templateId: "template-1",
      name: "Runbook Copy",
      category: "runbook",
    });
    await useKnowledgeStore.getState().deleteTemplate({
      projectId: "project-1",
      templateId: "template-2",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "http://localhost:7777/api/v1/projects/project-1/knowledge/assets?kind=template&q=runbook&category=runbook&source=system",
      expect.any(Object),
    );
    expect(created).toEqual(expect.objectContaining({ id: "template-2", kind: "template" }));
    expect(updated).toEqual(
      expect.objectContaining({
        id: "template-2",
        templateCategory: "runbook",
      }),
    );
    expect(duplicated).toEqual(
      expect.objectContaining({ id: "template-3", title: "Runbook Copy" }),
    );
    expect(useKnowledgeStore.getState().templates).toEqual([]);
  });

  it("handles ingested file upload and list", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "file-1",
            projectId: "project-1",
            kind: "ingested_file",
            title: "report.pdf",
            mimeType: "application/pdf",
            fileSize: 204800,
            ingestStatus: "ready",
            isPinned: false,
            version: 1,
            createdAt: "2026-03-26T12:00:00.000Z",
            updatedAt: "2026-03-26T12:00:00.000Z",
          },
        ])
      );

    await useKnowledgeStore.getState().fetchIngestedFiles("project-1");

    expect(useKnowledgeStore.getState().ingestedFiles).toHaveLength(1);
    expect(useKnowledgeStore.getState().ingestedFiles[0]).toEqual(
      expect.objectContaining({
        id: "file-1",
        title: "report.pdf",
        ingestStatus: "ready",
        kind: "ingested_file",
      }),
    );
  });

  it("searches knowledge and groups results by kind", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse({
        items: [
          {
            id: "page-1",
            kind: "wiki_page",
            title: "Runbook",
            snippet: "Operational…",
            updatedAt: "2026-03-26T12:00:00.000Z",
            score: 0.9,
          },
          {
            id: "file-1",
            kind: "ingested_file",
            title: "report.pdf",
            snippet: "",
            updatedAt: "2026-03-26T12:00:00.000Z",
            score: 0.7,
          },
        ],
        nextCursor: null,
      }),
    );

    await useKnowledgeStore.getState().searchKnowledge("project-1", "runbook");

    expect(useKnowledgeStore.getState().searchResults?.items).toHaveLength(2);
    expect(useKnowledgeStore.getState().searchResults?.items[0].kind).toBe("wiki_page");
  });

  it("selectWikiPageTree, selectIngestedFiles, selectTemplatesByCategory selectors work", () => {
    const tree: KnowledgeAssetTreeNode[] = [
      {
        id: "p1",
        projectId: "project-1",
        kind: "wiki_page",
        title: "Page",
        isPinned: false,
        version: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        children: [],
      },
    ];
    const templates: KnowledgeAsset[] = [
      {
        id: "t1",
        projectId: "project-1",
        kind: "template",
        title: "Runbook Template",
        templateCategory: "runbook",
        isPinned: false,
        version: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
      {
        id: "t2",
        projectId: "project-1",
        kind: "template",
        title: "ADR Template",
        templateCategory: "architecture",
        isPinned: false,
        version: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    ];
    const ingestedFiles: KnowledgeAsset[] = [
      {
        id: "f1",
        projectId: "project-1",
        kind: "ingested_file",
        title: "doc.pdf",
        isPinned: false,
        version: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    ];
    useKnowledgeStore.setState({ tree, templates, ingestedFiles });

    const state = useKnowledgeStore.getState();
    expect(selectWikiPageTree(state)).toHaveLength(1);
    expect(selectIngestedFiles(state)).toHaveLength(1);
    expect(selectTemplatesByCategory(state, "runbook")).toEqual([
      expect.objectContaining({ id: "t1" }),
    ]);
    expect(selectTemplatesByCategory(state)).toHaveLength(2);
  });
});
