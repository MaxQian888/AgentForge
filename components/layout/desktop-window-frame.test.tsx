"use client";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DesktopWindowFrame } from "./desktop-window-frame";

const closeMainWindowMock = jest.fn();
const getWindowChromeStateMock = jest.fn();
const minimizeMainWindowMock = jest.fn();
const subscribeWindowChromeStateMock = jest.fn();
const toggleMaximizeMainWindowMock = jest.fn();

let isDesktop = true;

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => ({
    isDesktop,
    closeMainWindow: closeMainWindowMock,
    getWindowChromeState: getWindowChromeStateMock,
    minimizeMainWindow: minimizeMainWindowMock,
    subscribeWindowChromeState: subscribeWindowChromeStateMock,
    toggleMaximizeMainWindow: toggleMaximizeMainWindowMock,
  }),
}));

describe("DesktopWindowFrame", () => {
  beforeEach(() => {
    isDesktop = true;
    closeMainWindowMock.mockReset();
    getWindowChromeStateMock.mockReset();
    minimizeMainWindowMock.mockReset();
    subscribeWindowChromeStateMock.mockReset();
    toggleMaximizeMainWindowMock.mockReset();
    getWindowChromeStateMock.mockResolvedValue({
      focused: true,
      maximized: false,
      minimized: false,
      visible: true,
    });
    subscribeWindowChromeStateMock.mockResolvedValue(jest.fn());
  });

  it("renders shared desktop titlebar controls in desktop mode", async () => {
    const user = userEvent.setup();

    render(
      <DesktopWindowFrame>
        <main>content</main>
      </DesktopWindowFrame>,
    );

    expect(screen.getByText("AgentForge")).toBeInTheDocument();
    expect(screen.getByText("Desktop Workspace")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Minimize window" }));
    await user.click(screen.getByRole("button", { name: "Maximize window" }));
    await user.click(screen.getByRole("button", { name: "Close window" }));

    expect(minimizeMainWindowMock).toHaveBeenCalledTimes(1);
    expect(toggleMaximizeMainWindowMock).toHaveBeenCalledTimes(1);
    expect(closeMainWindowMock).toHaveBeenCalledTimes(1);
  });

  it("keeps the frame wrapper but hides desktop-only controls on web", () => {
    isDesktop = false;

    render(
      <DesktopWindowFrame>
        <main>content</main>
      </DesktopWindowFrame>,
    );

    expect(screen.getByText("content")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Minimize window" }),
    ).not.toBeInTheDocument();
  });
});
