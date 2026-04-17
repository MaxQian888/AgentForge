import { act, renderHook } from "@testing-library/react";
import { useA11yPreferences } from "./use-a11y-preferences";
import {
  DEFAULT_APPEARANCE,
  useAppearanceStore,
} from "@/lib/stores/appearance-store";

type ChangeListener = () => void;

describe("useA11yPreferences", () => {
  const originalMatchMedia = window.matchMedia;
  let listeners: Map<string, Set<ChangeListener>>;
  let queryState: Record<string, boolean>;

  function installMatchMediaMock() {
    listeners = new Map();
    queryState = {
      "(prefers-reduced-motion: reduce)": false,
      "(prefers-contrast: more)": false,
    };
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: jest.fn().mockImplementation((query: string) => {
        const bucket = listeners.get(query) ?? new Set<ChangeListener>();
        listeners.set(query, bucket);
        return {
          get matches() {
            return queryState[query] ?? false;
          },
          media: query,
          onchange: null,
          addListener: jest.fn(),
          removeListener: jest.fn(),
          addEventListener: jest.fn(
            (_event: string, listener: ChangeListener) => {
              bucket.add(listener);
            },
          ),
          removeEventListener: jest.fn(
            (_event: string, listener: ChangeListener) => {
              bucket.delete(listener);
            },
          ),
          dispatchEvent: jest.fn(),
        };
      }),
    });
  }

  function emitChange(query: string) {
    act(() => {
      for (const listener of listeners.get(query) ?? []) {
        listener();
      }
    });
  }

  beforeEach(() => {
    localStorage.clear();
    useAppearanceStore.setState({ ...DEFAULT_APPEARANCE });
    installMatchMediaMock();
    document.documentElement.removeAttribute("data-density");
    document.documentElement.removeAttribute("data-contrast");
    document.documentElement.removeAttribute("data-reduced-motion");
    document.documentElement.removeAttribute("data-screen-reader");
  });

  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: originalMatchMedia,
    });
  });

  it("returns defaults when no system preference or override is set", () => {
    const { result } = renderHook(() => useA11yPreferences());
    expect(result.current.density).toBe("comfortable");
    expect(result.current.reducedMotionActive).toBe(false);
    expect(result.current.highContrast).toBe(false);
    expect(result.current.systemPrefersReducedMotion).toBe(false);
  });

  it("picks up prefers-reduced-motion from the OS when preference is system", () => {
    queryState["(prefers-reduced-motion: reduce)"] = true;
    const { result } = renderHook(() => useA11yPreferences());
    expect(result.current.reducedMotionActive).toBe(true);
    expect(result.current.systemPrefersReducedMotion).toBe(true);
  });

  it("user override wins over system preference", () => {
    queryState["(prefers-reduced-motion: reduce)"] = true;
    act(() => {
      useAppearanceStore.getState().setMotionPreference("allow");
    });
    const { result } = renderHook(() => useA11yPreferences());
    expect(result.current.systemPrefersReducedMotion).toBe(true);
    expect(result.current.reducedMotionActive).toBe(false);
  });

  it("forced reduce preference activates regardless of system state", () => {
    queryState["(prefers-reduced-motion: reduce)"] = false;
    act(() => {
      useAppearanceStore.getState().setMotionPreference("reduce");
    });
    const { result } = renderHook(() => useA11yPreferences());
    expect(result.current.reducedMotionActive).toBe(true);
  });

  it("reacts to system preference changes", () => {
    const { result } = renderHook(() => useA11yPreferences());
    expect(result.current.reducedMotionActive).toBe(false);
    act(() => {
      queryState["(prefers-reduced-motion: reduce)"] = true;
    });
    emitChange("(prefers-reduced-motion: reduce)");
    expect(result.current.reducedMotionActive).toBe(true);
  });

  it("high contrast respects user override and system preference", () => {
    queryState["(prefers-contrast: more)"] = true;
    const { result, rerender } = renderHook(() => useA11yPreferences());
    expect(result.current.highContrast).toBe(true);

    act(() => {
      useAppearanceStore.getState().setHighContrast(true);
    });
    rerender();
    expect(result.current.highContrast).toBe(true);
  });

  it("mirrors preferences to document.documentElement data attributes", () => {
    act(() => {
      useAppearanceStore.getState().setDensity("compact");
      useAppearanceStore.getState().setHighContrast(true);
      useAppearanceStore.getState().setScreenReaderMode(true);
      useAppearanceStore.getState().setMotionPreference("reduce");
    });
    renderHook(() => useA11yPreferences());
    const root = document.documentElement;
    expect(root.getAttribute("data-density")).toBe("compact");
    expect(root.getAttribute("data-contrast")).toBe("high");
    expect(root.getAttribute("data-reduced-motion")).toBe("true");
    expect(root.getAttribute("data-screen-reader")).toBe("true");
  });
});
