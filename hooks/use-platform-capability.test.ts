import { renderHook } from "@testing-library/react";

const runtimeMock = {
  isDesktop: true,
  selectFiles: jest.fn(),
  sendNotification: jest.fn(),
  updateTray: jest.fn(),
  registerShortcut: jest.fn(),
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
    get updateTray() {
      return runtimeMock.updateTray;
    },
    get registerShortcut() {
      return runtimeMock.registerShortcut;
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
  });

  it("exposes the shared platform runtime capabilities", () => {
    const { result } = renderHook(() => usePlatformCapability());

    expect(result.current).toEqual({
      isDesktop: true,
      selectFiles: runtimeMock.selectFiles,
      sendNotification: runtimeMock.sendNotification,
      updateTray: runtimeMock.updateTray,
      registerShortcut: runtimeMock.registerShortcut,
      checkForUpdate: runtimeMock.checkForUpdate,
      installUpdate: runtimeMock.installUpdate,
      relaunchToUpdate: runtimeMock.relaunchToUpdate,
      getDesktopRuntimeStatus: runtimeMock.getDesktopRuntimeStatus,
      getPluginRuntimeSummary: runtimeMock.getPluginRuntimeSummary,
      subscribeDesktopEvents: runtimeMock.subscribeDesktopEvents,
    });
  });

  it("reflects updated runtime flags on rerender", () => {
    const { result, rerender } = renderHook(() => usePlatformCapability());

    expect(result.current.isDesktop).toBe(true);

    runtimeMock.isDesktop = false;
    rerender();

    expect(result.current.isDesktop).toBe(false);
    expect(result.current.selectFiles).toBe(runtimeMock.selectFiles);
  });
});
