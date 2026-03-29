import { renderHook } from "@testing-library/react";

const runtimeMock = {
  isDesktop: true,
  selectFiles: jest.fn(),
  sendNotification: jest.fn(),
  syncNotificationTraySummary: jest.fn(),
  updateTray: jest.fn(),
  registerShortcut: jest.fn(),
  performShellAction: jest.fn(),
  closeMainWindow() {
    return runtimeMock.performShellAction({
      actionId: "close_main_window",
      source: "window",
    });
  },
  focusMainWindow: jest.fn(),
  getWindowChromeState: jest.fn(),
  minimizeMainWindow() {
    return runtimeMock.performShellAction({
      actionId: "minimize_main_window",
      source: "window",
    });
  },
  restoreMainWindow: jest.fn(),
  showMainWindow: jest.fn(),
  subscribeWindowChromeState(
    handler: (state: { maximized: boolean }) => void,
  ) {
    return runtimeMock.subscribeDesktopEvents(handler);
  },
  toggleMaximizeMainWindow: jest.fn(),
  checkForUpdate: jest.fn(),
  installUpdate: jest.fn(),
  relaunchToUpdate: jest.fn(),
  getDesktopRuntimeStatus: jest.fn(),
  getPluginRuntimeSummary: jest.fn(),
  subscribeDesktopEvents: jest.fn(),
};

jest.mock("@/lib/platform-runtime", () => ({
  platformRuntime: {
    get isDesktop() {
      return runtimeMock.isDesktop;
    },
    get selectFiles() {
      return runtimeMock.selectFiles;
    },
    get sendNotification() {
      return runtimeMock.sendNotification;
    },
    get syncNotificationTraySummary() {
      return runtimeMock.syncNotificationTraySummary;
    },
    get updateTray() {
      return runtimeMock.updateTray;
    },
    get registerShortcut() {
      return runtimeMock.registerShortcut;
    },
    get performShellAction() {
      return runtimeMock.performShellAction;
    },
    get closeMainWindow() {
      return runtimeMock.closeMainWindow;
    },
    get focusMainWindow() {
      return runtimeMock.focusMainWindow;
    },
    get getWindowChromeState() {
      return runtimeMock.getWindowChromeState;
    },
    get minimizeMainWindow() {
      return runtimeMock.minimizeMainWindow;
    },
    get restoreMainWindow() {
      return runtimeMock.restoreMainWindow;
    },
    get showMainWindow() {
      return runtimeMock.showMainWindow;
    },
    get subscribeWindowChromeState() {
      return runtimeMock.subscribeWindowChromeState;
    },
    get toggleMaximizeMainWindow() {
      return runtimeMock.toggleMaximizeMainWindow;
    },
    get checkForUpdate() {
      return runtimeMock.checkForUpdate;
    },
    get installUpdate() {
      return runtimeMock.installUpdate;
    },
    get relaunchToUpdate() {
      return runtimeMock.relaunchToUpdate;
    },
    get getDesktopRuntimeStatus() {
      return runtimeMock.getDesktopRuntimeStatus;
    },
    get getPluginRuntimeSummary() {
      return runtimeMock.getPluginRuntimeSummary;
    },
    get subscribeDesktopEvents() {
      return runtimeMock.subscribeDesktopEvents;
    },
  },
}));

import { usePlatformCapability } from "./use-platform-capability";

describe("usePlatformCapability", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    runtimeMock.isDesktop = true;
    runtimeMock.performShellAction.mockResolvedValue({
      actionId: "close_main_window",
      mode: "desktop",
      ok: true,
      status: "completed",
    });
    runtimeMock.subscribeDesktopEvents.mockResolvedValue(jest.fn());
  });

  it("exposes the shared platform runtime capabilities", () => {
    const { result } = renderHook(() => usePlatformCapability());

    expect(result.current.isDesktop).toBe(true);
    expect(result.current.selectFiles).toBe(runtimeMock.selectFiles);
    expect(result.current.sendNotification).not.toBe(
      runtimeMock.sendNotification,
    );
    expect(result.current.closeMainWindow).not.toBe(runtimeMock.closeMainWindow);
    expect(result.current.subscribeWindowChromeState).not.toBe(
      runtimeMock.subscribeWindowChromeState,
    );
  });

  it("reflects updated runtime flags on rerender", () => {
    const { result, rerender } = renderHook(() => usePlatformCapability());

    expect(result.current.isDesktop).toBe(true);

    runtimeMock.isDesktop = false;
    rerender();

    expect(result.current.isDesktop).toBe(false);
    expect(result.current.selectFiles).toBe(runtimeMock.selectFiles);
  });

  it("binds desktop helpers so destructured methods keep the runtime receiver", async () => {
    const { result } = renderHook(() => usePlatformCapability());
    const { closeMainWindow, minimizeMainWindow, subscribeWindowChromeState } =
      result.current;
    const handler = jest.fn();

    await closeMainWindow();
    await minimizeMainWindow();
    await subscribeWindowChromeState(handler);

    expect(runtimeMock.performShellAction).toHaveBeenNthCalledWith(1, {
      actionId: "close_main_window",
      source: "window",
    });
    expect(runtimeMock.performShellAction).toHaveBeenNthCalledWith(2, {
      actionId: "minimize_main_window",
      source: "window",
    });
    expect(runtimeMock.subscribeDesktopEvents).toHaveBeenCalledWith(handler);
  });
});
