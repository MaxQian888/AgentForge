jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useNotificationStore } from "./notification-store";

describe("useNotificationStore", () => {
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
    useNotificationStore.setState({
      notifications: [],
      unreadCount: 0,
    });
  });

  it("normalizes backend notification DTO fields into frontend shape", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "notification-1",
          targetId: "member-1",
          type: "task_progress_stalled",
          title: "Task stalled: Implement detector",
          body: "Task Implement detector is stalled.",
          data: JSON.stringify({ href: "/project?id=project-1#task-task-1" }),
          isRead: false,
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ])
    );

    await useNotificationStore.getState().fetchNotifications();

    expect(useNotificationStore.getState().notifications).toEqual([
      expect.objectContaining({
        id: "notification-1",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        message: "Task Implement detector is stalled.",
        href: "/project?id=project-1#task-task-1",
        read: false,
      }),
    ]);
    expect(useNotificationStore.getState().unreadCount).toBe(1);
  });

  it("upserts websocket replay notifications instead of duplicating them", () => {
    useNotificationStore.getState().addNotification({
      id: "notification-1",
      type: "task_progress_stalled",
      title: "Task stalled: Implement detector",
      body: "Task Implement detector is stalled.",
      createdAt: "2026-03-26T09:00:00.000Z",
      isRead: false,
    });

    useNotificationStore.getState().addNotification({
      id: "notification-1",
      type: "task_progress_stalled",
      title: "Task stalled: Implement detector",
      body: "Task Implement detector is stalled.",
      createdAt: "2026-03-26T09:00:00.000Z",
      isRead: true,
    });

    expect(useNotificationStore.getState().notifications).toHaveLength(1);
    expect(useNotificationStore.getState().notifications[0]).toEqual(
      expect.objectContaining({
        id: "notification-1",
        read: true,
      }),
    );
    expect(useNotificationStore.getState().unreadCount).toBe(0);
  });
});
