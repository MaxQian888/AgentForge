jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useTaskCommentStore } from "./task-comment-store";

describe("useTaskCommentStore", () => {
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
    useTaskCommentStore.setState({
      commentsByTask: {},
      loading: false,
      error: null,
    });
  });

  it("lists task comments and supports create/resolve/delete", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse([
          {
            id: "comment-1",
            taskId: "task-1",
            body: "First",
            mentions: ["alice"],
            createdBy: "user-1",
            createdAt: "2026-03-26T10:00:00.000Z",
            updatedAt: "2026-03-26T10:00:00.000Z",
          },
        ]),
      )
      .mockResolvedValueOnce(
        mockJsonResponse(
          {
            id: "comment-2",
            taskId: "task-1",
            body: "Second",
            mentions: [],
            createdBy: "user-1",
            createdAt: "2026-03-26T10:01:00.000Z",
            updatedAt: "2026-03-26T10:01:00.000Z",
          },
          201,
        ),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "comment-2",
          taskId: "task-1",
          body: "Second",
          mentions: [],
          resolvedAt: "2026-03-26T10:02:00.000Z",
          createdBy: "user-1",
          createdAt: "2026-03-26T10:01:00.000Z",
          updatedAt: "2026-03-26T10:02:00.000Z",
        }),
      )
      .mockResolvedValueOnce(mockJsonResponse({}))
      .mockResolvedValueOnce(mockJsonResponse([]));

    await useTaskCommentStore.getState().fetchComments("project-1", "task-1");
    expect(useTaskCommentStore.getState().commentsByTask["task-1"]).toHaveLength(1);

    await useTaskCommentStore.getState().createComment({
      projectId: "project-1",
      taskId: "task-1",
      body: "Second",
    });
    expect(useTaskCommentStore.getState().commentsByTask["task-1"]).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: "comment-2" })]),
    );

    await useTaskCommentStore.getState().setResolved({
      projectId: "project-1",
      taskId: "task-1",
      commentId: "comment-2",
      resolved: true,
    });
    expect(useTaskCommentStore.getState().commentsByTask["task-1"]).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: "comment-2", resolvedAt: "2026-03-26T10:02:00.000Z" })]),
    );

    await useTaskCommentStore.getState().deleteComment("project-1", "task-1", "comment-2");
    expect(useTaskCommentStore.getState().commentsByTask["task-1"]).toEqual([]);
  });
});
