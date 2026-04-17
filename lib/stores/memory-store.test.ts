jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { useMemoryStore } from "./memory-store";

type MockMemoryApiClient = {
  get: jest.Mock;
  post: jest.Mock;
  patch: jest.Mock;
  delete: jest.Mock;
};

function makeApiClient(): MockMemoryApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
    patch: jest.fn(),
    delete: jest.fn(),
  };
}

describe("useMemoryStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = useAuthStore.getState as unknown as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token" });
    useMemoryStore.setState({
      entries: [],
      loading: false,
    } as never);
  });

  it("loads workspace list and stats with canonical explorer filters", async () => {
    const api = makeApiClient();
    api.get
      .mockResolvedValueOnce({
        data: [
          {
            id: 123,
            projectId: "project-1",
            scope: "role",
            roleId: "reviewer",
            category: "episodic",
            key: "incident",
            content: "Handle review escalations",
            metadata: { source: "feedback" },
            metadataObject: { source: "feedback" },
            relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
            relevanceScore: "0.9",
            accessCount: "4",
            lastAccessedAt: "2026-03-25T10:05:00.000Z",
            createdAt: "2026-03-25T10:00:00.000Z",
            updatedAt: "2026-03-25T10:06:00.000Z",
          },
        ],
      })
      .mockResolvedValueOnce({
        data: {
          totalCount: 3,
          approxStorageBytes: 4096,
          byCategory: { episodic: 2, semantic: 1 },
          byScope: { role: 2, project: 1 },
          oldestCreatedAt: "2026-03-01T00:00:00.000Z",
          newestCreatedAt: "2026-03-25T10:00:00.000Z",
          lastAccessedAt: "2026-03-25T10:05:00.000Z",
        },
      });
    mockCreateApiClient.mockReturnValue(api);

    await useMemoryStore.getState().loadWorkspace("project-1", {
      query: "review",
      scope: "role",
      category: "episodic",
      roleId: "reviewer",
      startAt: "2026-03-01T00:00:00.000Z",
      endAt: "2026-03-31T23:59:59.000Z",
      limit: 50,
    });

    expect(api.get).toHaveBeenNthCalledWith(
      1,
      "/api/v1/projects/project-1/memory?query=review&scope=role&category=episodic&roleId=reviewer&startAt=2026-03-01T00%3A00%3A00.000Z&endAt=2026-03-31T23%3A59%3A59.000Z&limit=50",
      { token: "test-token" },
    );
    expect(api.get).toHaveBeenNthCalledWith(
      2,
      "/api/v1/projects/project-1/memory/stats?query=review&scope=role&category=episodic&roleId=reviewer&startAt=2026-03-01T00%3A00%3A00.000Z&endAt=2026-03-31T23%3A59%3A59.000Z&limit=50",
      { token: "test-token" },
    );

    expect(useMemoryStore.getState()).toMatchObject({
      currentProjectId: "project-1",
      filters: {
        query: "review",
        scope: "role",
        category: "episodic",
        roleId: "reviewer",
        tag: "",
        startAt: "2026-03-01T00:00:00.000Z",
        endAt: "2026-03-31T23:59:59.000Z",
        limit: 50,
      },
      loading: false,
      statsLoading: false,
      entries: [
        {
          id: "123",
          projectId: "project-1",
          scope: "role",
          roleId: "reviewer",
          category: "episodic",
          key: "incident",
          content: "Handle review escalations",
          metadata: JSON.stringify({ source: "feedback" }),
          metadataObject: { source: "feedback" },
          relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
          relevanceScore: 0.9,
          accessCount: 4,
          lastAccessedAt: "2026-03-25T10:05:00.000Z",
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:06:00.000Z",
        },
      ],
      stats: {
        totalCount: 3,
        approxStorageBytes: 4096,
        byCategory: { episodic: 2, semantic: 1 },
        byScope: { role: 2, project: 1 },
        oldestCreatedAt: "2026-03-01T00:00:00.000Z",
        newestCreatedAt: "2026-03-25T10:00:00.000Z",
        lastAccessedAt: "2026-03-25T10:05:00.000Z",
      },
    });
  });

  it("fetches memory detail for the selected entry", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: {
        id: "memory-1",
        projectId: "project-1",
        scope: "role",
        roleId: "reviewer",
        category: "episodic",
        key: "incident",
        content: "Full incident timeline",
        metadata: "{\"source\":\"feedback\"}",
        metadataObject: { source: "feedback", taskId: "task-1" },
        relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
        relevanceScore: 0.75,
        accessCount: 6,
        lastAccessedAt: "2026-03-25T11:00:00.000Z",
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:30:00.000Z",
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    await useMemoryStore.getState().fetchMemoryDetail(
      "project-1",
      "memory-1",
      "reviewer",
    );

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/memory-1?roleId=reviewer",
      { token: "test-token" },
    );
    expect(useMemoryStore.getState()).toMatchObject({
      detailLoading: false,
      detailError: null,
      detail: {
        id: "memory-1",
        key: "incident",
        content: "Full incident timeline",
        metadataObject: { source: "feedback", taskId: "task-1" },
        relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
      },
    });
  });

  it("bulk deletes selected memories and refreshes explorer truth", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({ data: { deletedCount: 2 } });
    api.get
      .mockResolvedValueOnce({
        data: [
          {
            id: "memory-3",
            projectId: "project-1",
            scope: "project",
            roleId: "",
            category: "semantic",
            key: "remaining",
            content: "Only one entry left",
            metadata: "{}",
            relevanceScore: 1,
            accessCount: 1,
            createdAt: "2026-03-25T12:00:00.000Z",
            updatedAt: "2026-03-25T12:00:00.000Z",
          },
        ],
      })
      .mockResolvedValueOnce({
        data: {
          totalCount: 1,
          approxStorageBytes: 512,
          byCategory: { semantic: 1 },
          byScope: { project: 1 },
        },
      });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "",
        scope: "all",
        category: "all",
        roleId: "",
        tag: "",
        startAt: "",
        endAt: "",
        limit: 20,
      },
      selectedMemoryIds: ["memory-1", "memory-2"],
      selectedMemoryId: "memory-1",
      detail: {
        id: "memory-1",
        projectId: "project-1",
        scope: "project",
        roleId: "",
        category: "episodic",
        key: "alpha",
        content: "Alpha",
        metadata: "{}",
        metadataObject: {},
        relatedContext: [],
        relevanceScore: 0.4,
        accessCount: 1,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:00:00.000Z",
      },
      entries: [
        {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "alpha",
          content: "Alpha",
          metadata: "{}",
          relevanceScore: 0.4,
          accessCount: 1,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:00:00.000Z",
        },
        {
          id: "memory-2",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "semantic",
          key: "beta",
          content: "Beta",
          metadata: "{}",
          relevanceScore: 0.5,
          accessCount: 2,
          createdAt: "2026-03-25T11:00:00.000Z",
          updatedAt: "2026-03-25T11:00:00.000Z",
        },
      ],
    } as never);

    await useMemoryStore.getState().bulkDeleteMemories("project-1", [
      "memory-1",
      "memory-2",
    ]);

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/bulk-delete",
      { ids: ["memory-1", "memory-2"], roleId: undefined },
      { token: "test-token" },
    );
    expect(useMemoryStore.getState()).toMatchObject({
      selectedMemoryIds: [],
      selectedMemoryId: null,
      detail: null,
      lastMutation: { type: "bulk-delete", deletedCount: 2 },
      entries: [expect.objectContaining({ id: "memory-3" })],
      stats: expect.objectContaining({ totalCount: 1 }),
    });
  });

  it("preserves explorer context when cleanup fails", async () => {
    const api = makeApiClient();
    api.post.mockRejectedValueOnce(new Error("cleanup failed"));
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "incident",
        scope: "all",
        category: "episodic",
        roleId: "",
        tag: "",
        startAt: "",
        endAt: "",
        limit: 20,
      },
      selectedMemoryId: "memory-1",
      entries: [
        {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "incident",
          content: "Keep this entry",
          metadata: "{}",
          relevanceScore: 0.8,
          accessCount: 3,
          createdAt: "2026-03-25T12:00:00.000Z",
          updatedAt: "2026-03-25T12:00:00.000Z",
        },
      ],
    } as never);

    await expect(
      useMemoryStore.getState().cleanupMemories("project-1", {
        retentionDays: 30,
      }),
    ).rejects.toThrow("cleanup failed");

    expect(useMemoryStore.getState()).toMatchObject({
      filters: expect.objectContaining({ query: "incident", category: "episodic" }),
      selectedMemoryId: "memory-1",
      entries: [expect.objectContaining({ id: "memory-1" })],
      actionError: "cleanup failed",
      actionLoading: false,
    });
  });

  it("exports the current filtered scope without mutating list state", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: {
        projectId: "project-1",
        exportedAt: "2026-03-25T12:30:00.000Z",
        entries: [
          {
            id: "memory-1",
            scope: "project",
            roleId: "",
            category: "episodic",
            key: "incident",
            content: "Full export payload",
            metadata: "{}",
            createdAt: "2026-03-25T10:00:00.000Z",
            updatedAt: "2026-03-25T10:30:00.000Z",
          },
        ],
      },
    });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "incident",
        scope: "all",
        category: "episodic",
        roleId: "",
        tag: "",
        startAt: "2026-03-01T00:00:00.000Z",
        endAt: "2026-03-31T23:59:59.000Z",
        limit: 20,
      },
      entries: [
        {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "incident",
          content: "Keep the list stable",
          metadata: "{}",
          relevanceScore: 0.8,
          accessCount: 3,
          createdAt: "2026-03-25T12:00:00.000Z",
          updatedAt: "2026-03-25T12:00:00.000Z",
        },
      ],
    } as never);

    const exported = await useMemoryStore.getState().exportMemories("project-1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/export?query=incident&category=episodic&startAt=2026-03-01T00%3A00%3A00.000Z&endAt=2026-03-31T23%3A59%3A59.000Z&limit=20",
      { token: "test-token" },
    );
    expect(exported).toEqual(
      expect.objectContaining({
        projectId: "project-1",
        entries: [expect.objectContaining({ id: "memory-1", key: "incident" })],
      }),
    );
    expect(useMemoryStore.getState().entries).toEqual([
      expect.objectContaining({ id: "memory-1", content: "Keep the list stable" }),
    ]);
  });

  it("stores a new memory entry and appends the normalized result", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({
      data: {
        id: "memory-1",
        projectId: "project-1",
        category: "episodic",
        kind: "operator_note",
        tags: ["ops", "release"],
        editable: true,
        key: "deployment",
        content: "Use staged rollout",
        metadata: { channel: "ops" },
      },
    });
    api.get
      .mockResolvedValueOnce({
        data: [
          {
            id: "memory-1",
            projectId: "project-1",
            scope: "project",
            roleId: "",
            category: "episodic",
            kind: "operator_note",
            tags: ["ops", "release"],
            editable: true,
            key: "deployment",
            content: "Use staged rollout",
            metadata: { channel: "ops", kind: "operator_note", tags: ["ops", "release"] },
            relevanceScore: 1,
            accessCount: 0,
            createdAt: "2026-03-25T12:00:00.000Z",
            updatedAt: "2026-03-25T12:00:00.000Z",
          },
        ],
      })
      .mockResolvedValueOnce({
        data: {
          totalCount: 1,
          approxStorageBytes: 256,
          byCategory: { episodic: 1 },
          byScope: { project: 1 },
        },
      });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "",
        scope: "all",
        category: "all",
        roleId: "",
        tag: "",
        startAt: "",
        endAt: "",
        limit: 20,
      },
    } as never);

    await useMemoryStore.getState().storeMemory("project-1", {
      key: "deployment",
      content: "Use staged rollout",
      scope: "project",
      roleId: "ops",
      category: "episodic",
      kind: "operator_note",
      tags: ["ops", "release"],
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory",
      {
        key: "deployment",
        content: "Use staged rollout",
        scope: "project",
        roleId: "ops",
        category: "episodic",
        tags: ["ops", "release"],
        metadata: JSON.stringify({
          kind: "operator_note",
          editable: true,
          tags: ["ops", "release"],
        }),
      },
      { token: "test-token" },
    );
    expect(useMemoryStore.getState().entries).toEqual([
      expect.objectContaining({
        id: "memory-1",
        projectId: "project-1",
        scope: "project",
        category: "episodic",
        kind: "operator_note",
        tags: ["ops", "release"],
        editable: true,
        metadata: JSON.stringify({
          channel: "ops",
          kind: "operator_note",
          tags: ["ops", "release"],
        }),
      }),
    ]);
  });

  it("updates a memory note, preserves tag filters, and exports a single entry", async () => {
    const api = makeApiClient();
    api.patch.mockResolvedValueOnce({
      data: {
        id: "memory-1",
        projectId: "project-1",
        scope: "project",
        roleId: "",
        category: "episodic",
        kind: "operator_note",
        tags: ["ops", "release"],
        editable: true,
        key: "release-note",
        content: "Pinned release checklist",
        metadata: { kind: "operator_note", editable: true, tags: ["ops", "release"] },
        metadataObject: { kind: "operator_note", editable: true, tags: ["ops", "release"] },
        relevanceScore: 1,
        accessCount: 1,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T12:00:00.000Z",
      },
    });
    api.get
      .mockResolvedValueOnce({
        data: [
          {
            id: "memory-1",
            projectId: "project-1",
            scope: "project",
            roleId: "",
            category: "episodic",
            kind: "operator_note",
            tags: ["ops", "release"],
            editable: true,
            key: "release-note",
            content: "Pinned release checklist",
            metadata: { kind: "operator_note", editable: true, tags: ["ops", "release"] },
            relevanceScore: 1,
            accessCount: 1,
            createdAt: "2026-03-25T10:00:00.000Z",
            updatedAt: "2026-03-25T12:00:00.000Z",
          },
        ],
      })
      .mockResolvedValueOnce({
        data: {
          totalCount: 1,
          approxStorageBytes: 300,
          byCategory: { episodic: 1 },
          byScope: { project: 1 },
        },
      })
      .mockResolvedValueOnce({
        data: {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          kind: "operator_note",
          tags: ["ops", "release"],
          editable: true,
          key: "release-note",
          content: "Pinned release checklist",
          metadata: { kind: "operator_note", editable: true, tags: ["ops", "release"] },
          metadataObject: { kind: "operator_note", editable: true, tags: ["ops", "release"] },
          relevanceScore: 1,
          accessCount: 1,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T12:00:00.000Z",
        },
      });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "",
        scope: "all",
        category: "all",
        roleId: "",
        tag: "ops",
        startAt: "",
        endAt: "",
        limit: 20,
      },
      selectedMemoryId: "memory-1",
    } as never);

    await useMemoryStore.getState().updateMemory("project-1", "memory-1", {
      key: "release-note",
      content: "Pinned release checklist",
      tags: ["ops", "release"],
    });

    expect(api.patch).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/memory-1",
      {
        key: "release-note",
        content: "Pinned release checklist",
        tags: ["ops", "release"],
      },
      { token: "test-token" },
    );
    expect(useMemoryStore.getState()).toMatchObject({
      filters: expect.objectContaining({ tag: "ops" }),
      detail: expect.objectContaining({
        id: "memory-1",
        kind: "operator_note",
        tags: ["ops", "release"],
        editable: true,
      }),
    });

    const exported = await useMemoryStore.getState().exportMemoryEntry(
      "project-1",
      "memory-1",
    );
    expect(exported).toEqual(
      expect.objectContaining({
        id: "memory-1",
        tags: ["ops", "release"],
      }),
    );
  });

  it("returns early when no auth token is available", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useMemoryStore.getState().loadWorkspace("project-1", { query: "review" });

    expect(mockCreateApiClient).not.toHaveBeenCalled();
    expect(useMemoryStore.getState()).toMatchObject({
      entries: [],
      loading: false,
    });
  });

  it("resets pagination to page 1 whenever filters change", () => {
    useMemoryStore.setState({
      filters: {
        query: "",
        scope: "all",
        category: "all",
        roleId: "",
        tag: "",
        startAt: "",
        endAt: "",
        limit: 20,
      },
      pagination: { page: 3, pageSize: 10 },
    } as never);

    useMemoryStore.getState().setFilters({ query: "incident" });

    expect(useMemoryStore.getState().pagination).toEqual({
      page: 1,
      pageSize: 10,
    });
  });

  it("updates page and pageSize via setPagination", () => {
    useMemoryStore.setState({
      pagination: { page: 1, pageSize: 10 },
    } as never);

    useMemoryStore.getState().setPagination({ page: 4 });
    expect(useMemoryStore.getState().pagination).toEqual({
      page: 4,
      pageSize: 10,
    });

    useMemoryStore.getState().setPagination({ pageSize: 25 });
    expect(useMemoryStore.getState().pagination).toEqual({
      page: 1,
      pageSize: 25,
    });
  });

  it("bulk deletes by criteria using the current entry window", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({ data: { deletedCount: 3 } });
    api.get
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({
        data: {
          totalCount: 0,
          approxStorageBytes: 0,
          byCategory: {},
          byScope: {},
        },
      });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      currentProjectId: "project-1",
      filters: {
        query: "",
        scope: "all",
        category: "all",
        roleId: "reviewer",
        tag: "",
        startAt: "",
        endAt: "",
        limit: 20,
      },
      entries: [
        {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "alpha",
          content: "alpha",
          metadata: "{}",
          relevanceScore: 1,
          accessCount: 0,
          createdAt: "2026-03-25T10:00:00.000Z",
        },
        {
          id: "memory-2",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "beta",
          content: "beta",
          metadata: "{}",
          relevanceScore: 1,
          accessCount: 0,
          createdAt: "2026-03-25T11:00:00.000Z",
        },
        {
          id: "memory-3",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "gamma",
          content: "gamma",
          metadata: "{}",
          relevanceScore: 1,
          accessCount: 0,
          createdAt: "2026-03-25T12:00:00.000Z",
        },
      ],
    } as never);

    const result = await useMemoryStore
      .getState()
      .bulkDeleteByCriteria("project-1");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/bulk-delete",
      {
        ids: ["memory-1", "memory-2", "memory-3"],
        roleId: "reviewer",
      },
      { token: "test-token" },
    );
    expect(result).toEqual({ deletedCount: 3 });
    expect(useMemoryStore.getState()).toMatchObject({
      selectedMemoryIds: [],
      lastMutation: { type: "bulk-delete-criteria", deletedCount: 3 },
    });
  });

  it("adds and removes memory tags by delegating to updateMemory", async () => {
    const api = makeApiClient();
    api.patch.mockResolvedValue({
      data: {
        id: "memory-1",
        projectId: "project-1",
        scope: "project",
        roleId: "",
        category: "episodic",
        kind: "operator_note",
        tags: ["ops", "pinned"],
        editable: true,
        key: "design-note",
        content: "body",
        metadata: "{}",
        metadataObject: {},
        relevanceScore: 1,
        accessCount: 0,
        createdAt: "2026-03-25T10:00:00.000Z",
      },
    });
    api.get.mockResolvedValue({ data: [] });
    mockCreateApiClient.mockReturnValue(api);

    const initialEntry = {
      id: "memory-1",
      projectId: "project-1",
      scope: "project" as const,
      roleId: "",
      category: "episodic" as const,
      kind: "operator_note",
      tags: ["ops"],
      editable: true,
      key: "design-note",
      content: "body",
      metadata: "{}",
      metadataObject: {},
      relatedContext: [],
      relevanceScore: 1,
      accessCount: 0,
      createdAt: "2026-03-25T10:00:00.000Z",
    };

    useMemoryStore.setState({
      currentProjectId: "project-1",
      entries: [initialEntry],
    } as never);

    await useMemoryStore
      .getState()
      .addMemoryTag("project-1", "memory-1", "pinned");

    expect(api.patch).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/memory-1",
      expect.objectContaining({ tags: ["ops", "pinned"] }),
      { token: "test-token" },
    );
    expect(useMemoryStore.getState().lastMutation).toEqual({
      type: "tag-added",
      deletedCount: 1,
    });

    useMemoryStore.setState({
      entries: [{ ...initialEntry, tags: ["ops", "pinned"] }],
    } as never);

    await useMemoryStore
      .getState()
      .removeMemoryTag("project-1", "memory-1", "ops");

    expect(api.patch).toHaveBeenLastCalledWith(
      "/api/v1/projects/project-1/memory/memory-1",
      expect.objectContaining({ tags: ["pinned"] }),
      { token: "test-token" },
    );
    expect(useMemoryStore.getState().lastMutation).toEqual({
      type: "tag-removed",
      deletedCount: 1,
    });
  });

  it("builds JSON and CSV export blobs", () => {
    const payload = {
      projectId: "project-1",
      exportedAt: "2026-03-25T12:00:00.000Z",
      entries: [
        {
          id: "memory-1",
          scope: "project",
          roleId: "",
          category: "episodic",
          key: "title",
          content: "body, with comma",
          tags: ["ops", "release"],
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T11:00:00.000Z",
        },
      ],
    };

    const json = useMemoryStore
      .getState()
      .buildExportBlob(payload as never, "json");
    expect(json.mimeType).toBe("application/json");
    expect(json.extension).toBe("json");
    expect(JSON.parse(json.content)).toEqual(payload);

    const csv = useMemoryStore
      .getState()
      .buildExportBlob(payload as never, "csv");
    expect(csv.mimeType).toBe("text/csv");
    expect(csv.extension).toBe("csv");
    const lines = csv.content.split("\n");
    expect(lines[0]).toBe(
      "id,scope,roleId,category,key,content,tags,createdAt,updatedAt",
    );
    expect(lines[1]).toContain("\"body, with comma\"");
    expect(lines[1]).toContain("ops|release");
  });
});
