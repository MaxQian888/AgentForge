jest.mock("@/components/ui/sidebar", () => ({
  SidebarTrigger: () => <button data-testid="sidebar-trigger" aria-label="Toggle Sidebar" />,
}));

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: jest.fn(),
}));

jest.mock("@/lib/stores/notification-store", () => ({
  useNotificationStore: jest.fn(),
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { Header } from "./header";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useNotificationStore } from "@/lib/stores/notification-store";

describe("Header", () => {
  it("shows notifications and lets the user mark them read and logout", async () => {
    const user = userEvent.setup();
    const logout = jest.fn().mockResolvedValue(undefined);
    const markRead = jest.fn();
    const markAllRead = jest.fn();
    const mockUseAuthStore = useAuthStore as unknown as jest.MockedFunction<typeof useAuthStore>;
    const mockUseNotificationStore =
      useNotificationStore as unknown as jest.MockedFunction<typeof useNotificationStore>;

    mockUseAuthStore.mockReturnValue({
      user: { name: "Alice Johnson" },
      logout,
    });
    mockUseNotificationStore.mockReturnValue({
      notifications: [
        { id: "n-1", title: "Task stalled", message: "Review queue blocked." },
      ],
      unreadCount: 1,
      markRead,
      markAllRead,
    });

    render(<Header />);

    expect(screen.getByTestId("sidebar-trigger")).toBeInTheDocument();
    expect(screen.getByText("AJ")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();

    const notificationTrigger = screen.getByText("1").closest("button");
    expect(notificationTrigger).not.toBeNull();

    await user.click(notificationTrigger!);
    await user.click(await screen.findByRole("button", { name: /Task stalled/i }));
    expect(markRead).toHaveBeenCalledWith("n-1");

    const accountTrigger = screen.getByText("AJ").closest("button");
    expect(accountTrigger).not.toBeNull();

    await user.click(accountTrigger!);
    await user.click(await screen.findByRole("menuitem", { name: /Logout/i }));
    expect(logout).toHaveBeenCalled();
  });
});
