"use client";

import { render, screen, waitFor } from "@testing-library/react";
import { DashboardShell } from "./dashboard-shell";
import { useAuthStore } from "@/lib/stores/auth-store";

const replaceMock = jest.fn();
const connectMock = jest.fn();
const disconnectMock = jest.fn();
const fetchNotificationsMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter() {
    return {
      replace: replaceMock,
      push: jest.fn(),
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
  useWSStore: (selector: (state: { connect: typeof connectMock; disconnect: typeof disconnectMock }) => unknown) =>
    selector({
      connect: connectMock,
      disconnect: disconnectMock,
    }),
}));

jest.mock("@/lib/stores/notification-store", () => ({
  useNotificationStore: (
    selector: (state: { fetchNotifications: typeof fetchNotificationsMock }) => unknown
  ) =>
    selector({
      fetchNotifications: fetchNotificationsMock,
    }),
}));

describe("DashboardShell", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    connectMock.mockReset();
    disconnectMock.mockReset();
    fetchNotificationsMock.mockReset();
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
      </DashboardShell>
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
      </DashboardShell>
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
      </DashboardShell>
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
      </DashboardShell>
    );

    await waitFor(() => {
      expect(connectMock).toHaveBeenCalledWith(
        "ws://localhost:7777/ws",
        "access-1"
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
      </DashboardShell>
    );

    await waitFor(() => {
      expect(fetchNotificationsMock).toHaveBeenCalledTimes(1);
    });
  });
});
