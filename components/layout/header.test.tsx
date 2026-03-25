jest.mock("./sidebar", () => ({
  MobileSidebar: () => <div data-testid="mobile-sidebar">mobile</div>,
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

    (useAuthStore as jest.Mock).mockReturnValue({
      user: { name: "Alice Johnson" },
      logout,
    });
    (useNotificationStore as jest.Mock).mockReturnValue({
      notifications: [
        { id: "n-1", title: "Task stalled", message: "Review queue blocked." },
      ],
      unreadCount: 1,
      markRead,
    });

    const { container } = render(<Header />);
    const buttons = container.querySelectorAll("button");

    expect(screen.getByTestId("mobile-sidebar")).toBeInTheDocument();
    expect(screen.getByText("AJ")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();

    await user.click(buttons[0]);
    await user.click(await screen.findByRole("button", { name: "Task stalled Review queue blocked." }));
    expect(markRead).toHaveBeenCalledWith("n-1");

    await user.click(buttons[1]);
    await user.click(await screen.findByRole("menuitem", { name: /Logout/i }));
    expect(logout).toHaveBeenCalled();
  });
});
