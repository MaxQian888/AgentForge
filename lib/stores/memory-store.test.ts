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
  delete: jest.Mock;
};

function makeApiClient(): MockMemoryApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
    delete: jest.fn(),
  };
}

describe("useMemoryStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = (useAuthStore.getState as unknown as jest.Mock);

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token" });
    useMemoryStore.setState({
      entries: [],
      loading: false,
    });
  });

  it("searches memory with query params and normalizes response entries", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
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
          relevanceScore: "0.9",
          accessCount: "4",
          createdAt: "2026-03-25T10:00:00.000Z",
        },
      ],
    });
    mockCreateApiClient.mockReturnValue(api);

    await useMemoryStore
      .getState()
      .searchMemory("project-1", "review", "role", "episodic");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory?query=review&scope=role&category=episodic",
      { token: "test-token" },
    );
    expect(useMemoryStore.getState()).toMatchObject({
      loading: false,
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
          relevanceScore: 0.9,
          accessCount: 4,
          createdAt: "2026-03-25T10:00:00.000Z",
        },
      ],
    });
  });

  it("resets loading even when searchMemory fails", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("network failure"));
    mockCreateApiClient.mockReturnValue(api);

    await expect(
      useMemoryStore.getState().searchMemory("project-1", "review"),
    ).rejects.toThrow("network failure");

    expect(useMemoryStore.getState().loading).toBe(false);
  });

  it("stores a new memory entry and appends the normalized result", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({
      data: {
        id: "memory-1",
        projectId: "project-1",
        key: "deployment",
        content: "Use staged rollout",
        metadata: { channel: "ops" },
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    await useMemoryStore.getState().storeMemory("project-1", {
      key: "deployment",
      content: "Use staged rollout",
      scope: "project",
      roleId: "ops",
      category: "procedural",
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory",
      {
        key: "deployment",
        content: "Use staged rollout",
        scope: "project",
        roleId: "ops",
        category: "procedural",
      },
      { token: "test-token" },
    );
    expect(useMemoryStore.getState().entries).toEqual([
      expect.objectContaining({
        id: "memory-1",
        projectId: "project-1",
        scope: "project",
        category: "semantic",
        metadata: JSON.stringify({ channel: "ops" }),
      }),
    ]);
    expect(useMemoryStore.getState().entries[0]?.createdAt).toEqual(
      expect.any(String),
    );
  });

  it("deletes a memory entry from local state after the API succeeds", async () => {
    const api = makeApiClient();
    api.delete.mockResolvedValueOnce({ data: null });
    mockCreateApiClient.mockReturnValue(api);
    useMemoryStore.setState({
      entries: [
        {
          id: "memory-1",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "semantic",
          key: "alpha",
          content: "Alpha",
          metadata: "",
          relevanceScore: 0,
          accessCount: 0,
          createdAt: "2026-03-25T10:00:00.000Z",
        },
        {
          id: "memory-2",
          projectId: "project-1",
          scope: "project",
          roleId: "",
          category: "semantic",
          key: "beta",
          content: "Beta",
          metadata: "",
          relevanceScore: 0,
          accessCount: 0,
          createdAt: "2026-03-25T10:05:00.000Z",
        },
      ],
    });

    await useMemoryStore.getState().deleteMemory("project-1", "memory-1");

    expect(api.delete).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/memory/memory-1",
      { token: "test-token" },
    );
    expect(useMemoryStore.getState().entries).toEqual([
      expect.objectContaining({ id: "memory-2" }),
    ]);
  });

  it("returns early when no auth token is available", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useMemoryStore.getState().searchMemory("project-1", "review");

    expect(mockCreateApiClient).not.toHaveBeenCalled();
    expect(useMemoryStore.getState()).toMatchObject({
      entries: [],
      loading: false,
    });
  });
});
