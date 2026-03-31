import { act, renderHook, waitFor } from "@testing-library/react";
import { useBreakpoint } from "./use-breakpoint";

type MatchMediaChangeListener = () => void;

describe("useBreakpoint", () => {
  const originalMatchMedia = window.matchMedia;
  const originalInnerWidth = window.innerWidth;

  let changeListeners: Set<MatchMediaChangeListener>;
  let addEventListenerMock: jest.Mock;
  let removeEventListenerMock: jest.Mock;

  function setViewportWidth(width: number) {
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      writable: true,
      value: width,
    });
  }

  function installMatchMediaMock() {
    changeListeners = new Set<MatchMediaChangeListener>();
    addEventListenerMock = jest.fn((_event: string, listener: MatchMediaChangeListener) => {
      changeListeners.add(listener);
    });
    removeEventListenerMock = jest.fn(
      (_event: string, listener: MatchMediaChangeListener) => {
        changeListeners.delete(listener);
      },
    );

    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches:
          (query === "(max-width: 767px)" && window.innerWidth < 768) ||
          (query === "(min-width: 768px) and (max-width: 1279px)" &&
            window.innerWidth >= 768 &&
            window.innerWidth < 1280) ||
          (query === "(min-width: 1280px)" && window.innerWidth >= 1280),
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: addEventListenerMock,
        removeEventListener: removeEventListenerMock,
        dispatchEvent: jest.fn(),
      })),
    });
  }

  function emitChange() {
    act(() => {
      for (const listener of changeListeners) {
        listener();
      }
    });
  }

  beforeEach(() => {
    setViewportWidth(1024);
    installMatchMediaMock();
  });

  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: originalMatchMedia,
    });
    setViewportWidth(originalInnerWidth);
  });

  it("reports the current named breakpoint and derived flags", async () => {
    setViewportWidth(640);
    installMatchMediaMock();

    const { result } = renderHook(() => useBreakpoint());

    await waitFor(() => {
      expect(result.current.breakpoint).toBe("mobile");
      expect(result.current.isMobile).toBe(true);
      expect(result.current.isTablet).toBe(false);
      expect(result.current.isDesktop).toBe(false);
    });
  });

  it("updates when the viewport crosses mobile, tablet, and desktop breakpoints", async () => {
    const { result } = renderHook(() => useBreakpoint());

    await waitFor(() => {
      expect(result.current.breakpoint).toBe("tablet");
      expect(result.current.isTablet).toBe(true);
    });

    setViewportWidth(1400);
    emitChange();

    await waitFor(() => {
      expect(result.current.breakpoint).toBe("desktop");
      expect(result.current.isDesktop).toBe(true);
      expect(result.current.isTablet).toBe(false);
    });

    setViewportWidth(700);
    emitChange();

    await waitFor(() => {
      expect(result.current.breakpoint).toBe("mobile");
      expect(result.current.isMobile).toBe(true);
      expect(result.current.isDesktop).toBe(false);
    });
  });

  it("removes registered media query listeners on unmount", () => {
    const { unmount } = renderHook(() => useBreakpoint());

    unmount();

    expect(addEventListenerMock).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );
    expect(removeEventListenerMock).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );
  });
});
