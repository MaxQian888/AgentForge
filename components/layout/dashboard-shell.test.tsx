"use client";

import { render, screen, waitFor } from "@testing-library/react";
import { DashboardShell } from "./dashboard-shell";
import { useAuthStore } from "@/lib/stores/auth-store";

const replaceMock = jest.fn();
const pushMock = jest.fn();
const connectMock = jest.fn();
const disconnectMock = jest.fn();
const fetchNotificationsMock = jest.fn();
const sendNotificationMock = jest.fn();
const syncNotificationTraySummaryMock = jest.fn();
const subscribeDesktopEventsMock = jest.fn();
const notificationStoreState = {
  fetchNotifications: fetchNotificationsMock,
  notifications: [] as Array<{
    id: string;
    type: string;
    title: string;
    message: string;
    data?: string;
    href?: string | null;
    read: boolean;
    createdAt: string;
  }>,
  unreadCount: 0,
};

jest.mock("next/navigation", () => ({
  useRouter() {
    return {
      replace: replaceMock,
      push: pushMock,
      prefetch: jest.fn(),
      back: jest.fn(),
    };
  },
}));

jest.mock("@/components/layout/sidebar", () => ({
  Sidebar: () => <div data-testid="sidebar">sidebar</div>,
}));

jest.mock("@/components/layout/header", () => ({
  Header: () => <div data-testid="header">header</div>,
}));

jest.mock("@/lib/backend-url", () => ({
  resolveBackendUrl: jest.fn().mockResolvedValue("http://localhost:7777"),
}));

jest.mock("@/lib/stores/ws-store", () => ({
  useWSStore: (
    selector: (state: {
      connect: typeof connectMock;
      disconnect: typeof disconnectMock;
    }) => unknown,
  ) =>
    selector({
      connect: connectMock,
      disconnect: disconnectMock,
    }),
}));

jest.mock("@/lib/stores/notification-store", () => ({
  useNotificationStore: (
    selector: (state: typeof notificationStoreState) => unknown,
  ) => selector(notificationStoreState),
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => ({
    isDesktop: true,
    sendNotification: sendNotificationMock,
    subscribeDesktopEvents: subscribeDesktopEventsMock,
    syncNotificationTraySummary: syncNotificationTraySummaryMock,
  }),
}));

describe("DashboardShell", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    pushMock.mockReset();
    connectMock.mockReset();
    disconnectMock.mockReset();
    fetchNotificationsMock.mockReset();
    sendNotificationMock.mockReset();
    syncNotificationTraySummaryMock.mockReset();
    subscribeDesktopEventsMock.mockReset();
    subscribeDesktopEventsMock.mockResolvedValue(jest.fn());
    notificationStoreState.notifications = [];
    notificationStoreState.unreadCount = 0;
    localStorage.clear();
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "idle",
      hasHydrated: true,
    } as never);
  });

  it("waits for auth resolution before rendering protected content", () => {
    useAuthStore.setState({ status: "checking", hasHydrated: true } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    expect(screen.queryByText("secret dashboard")).not.toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("redirects to login after auth resolves unauthenticated", async () => {
    useAuthStore.setState({
      status: "unauthenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/login");
    });
    expect(screen.queryByText("secret dashboard")).not.toBeInTheDocument();
  });

  it("renders protected content after auth resolves authenticated", () => {
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    expect(screen.getByText("secret dashboard")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("connects the websocket session after auth resolves authenticated", async () => {
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    const { unmount } = render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(connectMock).toHaveBeenCalledWith(
        "ws://localhost:7777/ws",
        "access-1",
      );
    });

    unmount();

    expect(disconnectMock).toHaveBeenCalled();
  });

  it("fetches notifications after the dashboard shell authenticates", async () => {
    fetchNotificationsMock.mockResolvedValue(undefined);
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(fetchNotificationsMock).toHaveBeenCalledTimes(1);
    });
  });

  it("bridges unread notifications to desktop delivery and tray summary without duplicate sends on rerender", async () => {
    sendNotificationMock.mockResolvedValue({
      mode: "desktop",
      notificationId: "notification-1",
      ok: true,
      status: "delivered",
    });
    syncNotificationTraySummaryMock.mockResolvedValue({
      mode: "desktop",
      ok: true,
    });
    notificationStoreState.notifications = [
      {
        id: "notification-1",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        message: "Task Implement detector is stalled.",
        href: "/project?id=project-1#task-task-1",
        read: false,
        createdAt: "2026-03-26T09:00:00.000Z",
      },
    ];
    notificationStoreState.unreadCount = 1;
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    const { rerender } = render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(sendNotificationMock).toHaveBeenCalledWith(
        expect.objectContaining({
          notificationId: "notification-1",
          type: "task_progress_stalled",
          title: "Task stalled: Implement detector",
          body: "Task Implement detector is stalled.",
        }),
      );
    });
    expect(syncNotificationTraySummaryMock).toHaveBeenCalledWith({
      latestTitle: "Task stalled: Implement detector",
      unreadCount: 1,
    });

    notificationStoreState.notifications = [
      {
        id: "notification-1",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        message: "Task Implement detector is stalled.",
        href: "/project?id=project-1#task-task-1",
        read: false,
        createdAt: "2026-03-26T09:00:00.000Z",
      },
    ];
    rerender(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(sendNotificationMock).toHaveBeenCalledTimes(1);
    });
  });

  it("suppresses foreground delivery when the notification policy requests it", async () => {
    syncNotificationTraySummaryMock.mockResolvedValue({
      mode: "desktop",
      ok: true,
    });
    notificationStoreState.notifications = [
      {
        id: "notification-2",
        type: "task_progress_stalled",
        title: "Task stalled: Implement detector",
        message: "Task Implement detector is stalled.",
        data: JSON.stringify({
          deliveryPolicy: "suppress_if_focused",
          href: "/project?id=project-1#task-task-1",
        }),
        href: "/project?id=project-1#task-task-1",
        read: false,
        createdAt: "2026-03-26T09:10:00.000Z",
      },
    ];
    notificationStoreState.unreadCount = 1;
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "visible",
    });

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(syncNotificationTraySummaryMock).toHaveBeenCalledWith({
        latestTitle: "Task stalled: Implement detector",
        unreadCount: 1,
      });
    });
    expect(sendNotificationMock).not.toHaveBeenCalled();
  });

  it("bridges wiki notifications through the same desktop delivery flow", async () => {
    sendNotificationMock.mockResolvedValue({
      mode: "desktop",
      notificationId: "notification-docs-1",
      ok: true,
      status: "delivered",
    });
    syncNotificationTraySummaryMock.mockResolvedValue({
      mode: "desktop",
      ok: true,
    });
    notificationStoreState.notifications = [
      {
        id: "notification-docs-1",
        type: "wiki.comment.mention",
        title: "Mentioned in Runbook",
        message: "Alice mentioned you in the Runbook page.",
        href: "/docs/page-1#comment-comment-1",
        read: false,
        createdAt: "2026-03-26T11:00:00.000Z",
      },
    ];
    notificationStoreState.unreadCount = 1;
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(sendNotificationMock).toHaveBeenCalledWith(
        expect.objectContaining({
          notificationId: "notification-docs-1",
          type: "wiki.comment.mention",
          title: "Mentioned in Runbook",
          body: "Alice mentioned you in the Runbook page.",
          href: "/docs/page-1#comment-comment-1",
        }),
      );
    });
  });

  it("bridges wiki page update notifications through desktop delivery", async () => {
    sendNotificationMock.mockResolvedValue({
      mode: "desktop",
      notificationId: "notification-docs-2",
      ok: true,
      status: "delivered",
    });
    syncNotificationTraySummaryMock.mockResolvedValue({
      mode: "desktop",
      ok: true,
    });
    notificationStoreState.notifications = [
      {
        id: "notification-docs-2",
        type: "wiki.page.updated",
        title: "Runbook updated",
        message: "The production runbook received a new version.",
        href: "/docs/page-1",
        read: false,
        createdAt: "2026-03-26T11:30:00.000Z",
      },
    ];
    notificationStoreState.unreadCount = 1;
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(sendNotificationMock).toHaveBeenCalledWith(
        expect.objectContaining({
          notificationId: "notification-docs-2",
          type: "wiki.page.updated",
          title: "Runbook updated",
          body: "The production runbook received a new version.",
          href: "/docs/page-1",
        }),
      );
    });
  });

  it("routes normalized shell action events through the router handoff", async () => {
    subscribeDesktopEventsMock.mockImplementation(
      async (
        handler: (event: {
          type: string;
          actionId?: string;
          href?: string;
        }) => void,
      ) => {
        handler({
          type: "shell.action",
          actionId: "open_notification_target",
          href: "/reviews?id=review-1",
        });
        return jest.fn();
      },
    );
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(
      <DashboardShell>
        <div>secret dashboard</div>
      </DashboardShell>,
    );

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/reviews?id=review-1");
    });
  });
});
