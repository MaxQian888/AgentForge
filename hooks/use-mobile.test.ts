import { act, renderHook, waitFor } from "@testing-library/react";
import { useIsMobile } from "./use-mobile";

type MatchMediaChangeListener = () => void;

describe("useIsMobile", () => {
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
    addEventListenerMock = jest.fn(
      (_event: string, listener: EventListenerOrEventListenerObject) => {
        changeListeners.add(listener as MatchMediaChangeListener);
      },
    );
    removeEventListenerMock = jest.fn(
      (_event: string, listener: EventListenerOrEventListenerObject) => {
        changeListeners.delete(listener as MatchMediaChangeListener);
      },
    );

    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches: window.innerWidth < 768,
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

  it("reports mobile when the viewport starts below the breakpoint", async () => {
    setViewportWidth(767);
    installMatchMediaMock();

    const { result } = renderHook(() => useIsMobile());

    await waitFor(() => {
      expect(result.current).toBe(true);
    });

    expect(window.matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
  });

  it("updates when the viewport crosses the mobile breakpoint", async () => {
    const { result } = renderHook(() => useIsMobile());

    await waitFor(() => {
      expect(result.current).toBe(false);
    });

    setViewportWidth(640);
    emitChange();

    await waitFor(() => {
      expect(result.current).toBe(true);
    });

    setViewportWidth(1024);
    emitChange();

    await waitFor(() => {
      expect(result.current).toBe(false);
    });
  });

  it("removes the media query listener on unmount", () => {
    const { unmount } = renderHook(() => useIsMobile());

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
