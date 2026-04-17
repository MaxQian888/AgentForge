"use client";

import { act, renderHook } from "@testing-library/react";
import {
  DEFAULT_APPEARANCE,
  useAppearanceStore,
} from "./appearance-store";

beforeEach(() => {
  localStorage.clear();
  useAppearanceStore.setState({ ...DEFAULT_APPEARANCE });
});

describe("useAppearanceStore", () => {
  it("defaults to system theme", () => {
    const { result } = renderHook(() => useAppearanceStore());
    expect(result.current.theme).toBe("system");
  });

  it("setTheme updates theme to dark", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setTheme("dark");
    });
    expect(result.current.theme).toBe("dark");
  });

  it("setTheme updates theme to light", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setTheme("light");
    });
    expect(result.current.theme).toBe("light");
  });

  it("setTheme updates theme back to system", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setTheme("dark");
    });
    act(() => {
      result.current.setTheme("system");
    });
    expect(result.current.theme).toBe("system");
  });

  it("defaults to comfortable density", () => {
    const { result } = renderHook(() => useAppearanceStore());
    expect(result.current.density).toBe("comfortable");
  });

  it("setDensity switches between compact, comfortable and spacious", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setDensity("compact");
    });
    expect(result.current.density).toBe("compact");
    act(() => {
      result.current.setDensity("spacious");
    });
    expect(result.current.density).toBe("spacious");
    act(() => {
      result.current.setDensity("comfortable");
    });
    expect(result.current.density).toBe("comfortable");
  });

  it("defaults motion preference to system", () => {
    const { result } = renderHook(() => useAppearanceStore());
    expect(result.current.motionPreference).toBe("system");
  });

  it("setMotionPreference toggles between system, reduce and allow", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setMotionPreference("reduce");
    });
    expect(result.current.motionPreference).toBe("reduce");
    act(() => {
      result.current.setMotionPreference("allow");
    });
    expect(result.current.motionPreference).toBe("allow");
  });

  it("high contrast defaults to false and toggles via setter", () => {
    const { result } = renderHook(() => useAppearanceStore());
    expect(result.current.highContrast).toBe(false);
    act(() => {
      result.current.setHighContrast(true);
    });
    expect(result.current.highContrast).toBe(true);
  });

  it("screen reader mode defaults to false and toggles via setter", () => {
    const { result } = renderHook(() => useAppearanceStore());
    expect(result.current.screenReaderMode).toBe(false);
    act(() => {
      result.current.setScreenReaderMode(true);
    });
    expect(result.current.screenReaderMode).toBe(true);
  });

  it("resetAppearance restores defaults", () => {
    const { result } = renderHook(() => useAppearanceStore());
    act(() => {
      result.current.setTheme("dark");
      result.current.setDensity("spacious");
      result.current.setMotionPreference("reduce");
      result.current.setHighContrast(true);
      result.current.setScreenReaderMode(true);
    });
    act(() => {
      result.current.resetAppearance();
    });
    expect(result.current).toMatchObject(DEFAULT_APPEARANCE);
  });
});
