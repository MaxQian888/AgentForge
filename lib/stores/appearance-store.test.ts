"use client";

import { act, renderHook } from "@testing-library/react";
import { useAppearanceStore } from "./appearance-store";

beforeEach(() => {
  localStorage.clear();
  useAppearanceStore.setState({ theme: "system" });
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
});
