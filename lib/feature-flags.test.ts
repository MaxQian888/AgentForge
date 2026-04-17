/**
 * @jest-environment jsdom
 */
import { act, renderHook } from "@testing-library/react";
import {
  DEFAULT_FEATURE_FLAGS,
  clearFeatureFlagOverrides,
  getFeatureFlag,
  isFeatureEnabled,
  resolveFeatureFlags,
  setFeatureFlagOverride,
  useFeatureFlag,
} from "@/lib/feature-flags";

describe("lib/feature-flags", () => {
  afterEach(() => {
    clearFeatureFlagOverrides();
    // Reset env vars we touched.
    delete process.env.NEXT_PUBLIC_FEATURE_WORKFLOW_BUILDER;
    delete process.env.NEXT_PUBLIC_FEATURE_MEMORY_EXPLORER;
    delete process.env.NEXT_PUBLIC_FEATURE_IM_BRIDGE_PANEL;
  });

  describe("isFeatureEnabled", () => {
    it("returns the hard-coded default when nothing else is set", () => {
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(true);
      expect(isFeatureEnabled("MEMORY_EXPLORER")).toBe(true);
    });

    it("honours the injected flags map over defaults", () => {
      expect(
        isFeatureEnabled("WORKFLOW_BUILDER", { WORKFLOW_BUILDER: false }),
      ).toBe(false);
    });

    it("honours runtime overrides over env/defaults", () => {
      setFeatureFlagOverride("WORKFLOW_BUILDER", false);
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(false);
      setFeatureFlagOverride("WORKFLOW_BUILDER", true);
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(true);
    });

    it("clears an override when passed null", () => {
      setFeatureFlagOverride("WORKFLOW_BUILDER", false);
      setFeatureFlagOverride("WORKFLOW_BUILDER", null);
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(
        DEFAULT_FEATURE_FLAGS.WORKFLOW_BUILDER,
      );
    });
  });

  describe("resolveFeatureFlags", () => {
    it("returns a snapshot merging defaults with injected overrides", () => {
      const snap = resolveFeatureFlags({ WORKFLOW_BUILDER: false });
      expect(snap.WORKFLOW_BUILDER).toBe(false);
      expect(snap.MEMORY_EXPLORER).toBe(true);
    });
  });

  describe("getFeatureFlag", () => {
    it("returns the resolved boolean for a flag", () => {
      setFeatureFlagOverride("MEMORY_EXPLORER", false);
      expect(getFeatureFlag("MEMORY_EXPLORER")).toBe(false);
    });
  });

  describe("useFeatureFlag", () => {
    it("reflects the current default", () => {
      const { result } = renderHook(() => useFeatureFlag("IM_BRIDGE_PANEL"));
      expect(result.current).toBe(true);
    });

    it("re-renders when a runtime override changes", () => {
      const { result } = renderHook(() => useFeatureFlag("IM_BRIDGE_PANEL"));
      expect(result.current).toBe(true);

      act(() => {
        setFeatureFlagOverride("IM_BRIDGE_PANEL", false);
      });
      expect(result.current).toBe(false);

      act(() => {
        setFeatureFlagOverride("IM_BRIDGE_PANEL", null);
      });
      expect(result.current).toBe(true);
    });
  });

  describe("parsing accepted env values", () => {
    // We exercise the env path indirectly by round-tripping through
    // isFeatureEnabled after mutating process.env.  The implementation
    // reads process.env on each call.
    const truthy = ["1", "true", "on", "yes", "TRUE", " 1 "];
    const falsy = ["0", "false", "off", "no", "FALSE", " 0 "];

    it.each(truthy)("treats %p as enabled", (value) => {
      process.env.NEXT_PUBLIC_FEATURE_WORKFLOW_BUILDER = value;
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(true);
    });

    it.each(falsy)("treats %p as disabled", (value) => {
      process.env.NEXT_PUBLIC_FEATURE_WORKFLOW_BUILDER = value;
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(false);
    });

    it("ignores garbage env values and falls back to the default", () => {
      process.env.NEXT_PUBLIC_FEATURE_WORKFLOW_BUILDER = "maybe";
      expect(isFeatureEnabled("WORKFLOW_BUILDER")).toBe(
        DEFAULT_FEATURE_FLAGS.WORKFLOW_BUILDER,
      );
    });
  });
});
