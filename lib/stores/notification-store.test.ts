jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { useNotificationStore } from "./notification-store";

const authStoreModule = jest.requireMock("@/lib/stores/auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

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
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
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

  it("falls back when notification metadata is missing or malformed", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "notification-2",
          targetId: "member-2",
          type: "info",
          title: "Broken metadata",
          data: "{not-json",
          read: true,
          createdAt: "2026-03-24T13:00:00.000Z",
        },
      ]),
    );

    await useNotificationStore.getState().fetchNotifications();

    expect(useNotificationStore.getState().notifications).toEqual([
      expect.objectContaining({
        id: "notification-2",
        message: "",
        href: null,
        read: true,
      }),
    ]);
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

  it("marks all notifications as read through the canonical endpoint", () => {
    useNotificationStore.setState({
      notifications: [
        {
          id: "notification-1",
          type: "task_progress_stalled",
          title: "Task stalled",
          message: "Task stalled.",
          read: false,
          createdAt: "2026-03-26T09:00:00.000Z",
        },
      ],
      unreadCount: 1,
    });

    fetchMock.mockResolvedValueOnce(mockJsonResponse({ message: "notifications marked as read" }));

    useNotificationStore.getState().markAllRead();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/notifications/read-all",
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Authorization: "Bearer test-token",
        }),
      }),
    );
    expect(useNotificationStore.getState().notifications[0]?.read).toBe(true);
    expect(useNotificationStore.getState().unreadCount).toBe(0);
  });

  it("marks a single notification as read and keeps non-matching entries intact", () => {
    useNotificationStore.setState({
      notifications: [
        {
          id: "notification-1",
          type: "task_progress_stalled",
          title: "Task stalled",
          message: "Task stalled.",
          read: false,
          createdAt: "2026-03-26T09:00:00.000Z",
        },
        {
          id: "notification-2",
          type: "review.completed",
          title: "Review complete",
          message: "Review complete.",
          read: false,
          createdAt: "2026-03-26T09:05:00.000Z",
        },
      ],
      unreadCount: 2,
    });

    fetchMock.mockResolvedValueOnce(mockJsonResponse({}));
    useNotificationStore.getState().markRead("notification-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/notifications/notification-1/read",
      expect.objectContaining({
        method: "PUT",
      }),
    );
    expect(useNotificationStore.getState()).toMatchObject({
      unreadCount: 1,
      notifications: [
        expect.objectContaining({ id: "notification-1", read: true }),
        expect.objectContaining({ id: "notification-2", read: false }),
      ],
    });
  });

  it("returns early without an auth token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useNotificationStore.getState().fetchNotifications();
    useNotificationStore.getState().markRead("notification-1");
    useNotificationStore.getState().markAllRead();

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
