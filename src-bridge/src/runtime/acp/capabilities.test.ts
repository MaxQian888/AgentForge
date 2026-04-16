import { describe, expect, test } from "bun:test";
import { gateUnstable, liveControlsFor } from "./capabilities.js";
import { AcpCapabilityUnsupported } from "./errors.js";

describe("capabilities.gateUnstable", () => {
  test("passes when capability advertised", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = { sessionCapabilities: { fork: { v1: true } } } as any;
    expect(() => gateUnstable(caps, "fork", "sessionCapabilities.fork")).not.toThrow();
  });

  test("throws AcpCapabilityUnsupported when missing", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = { sessionCapabilities: {} } as any;
    expect(() => gateUnstable(caps, "fork", "sessionCapabilities.fork")).toThrow(
      AcpCapabilityUnsupported,
    );
  });

  test("dotted path navigation", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = { a: { b: { c: true } } } as any;
    expect(() => gateUnstable(caps, "x", "a.b.c")).not.toThrow();
    expect(() => gateUnstable(caps, "x", "a.b.d")).toThrow();
  });

  test("false leaf counts as unsupported", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = { sessionCapabilities: { fork: false } } as any;
    expect(() => gateUnstable(caps, "fork", "sessionCapabilities.fork")).toThrow(
      AcpCapabilityUnsupported,
    );
  });

  test("error carries method and reason_code path", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = {} as any;
    try {
      gateUnstable(caps, "setModel", "sessionCapabilities.setModel");
      throw new Error("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(AcpCapabilityUnsupported);
      expect((err as AcpCapabilityUnsupported).method).toBe("setModel");
      expect((err as AcpCapabilityUnsupported).reason).toBe(
        "no_capability_sessionCapabilities.setModel",
      );
    }
  });
});

describe("capabilities.liveControlsFor", () => {
  const baseCaps = {
    mcpCapabilities: {},
    promptCapabilities: {},
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any;

  test("populates setModel when availableModels non-empty", () => {
    const lc = liveControlsFor({
      caps: baseCaps,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      availableModels: [{ modelId: "claude-opus" } as any],
    });
    expect(lc.setModel).toBe(true);
    expect(lc.setThinkingBudget).toBe(false);
  });

  test("sets setThinkingBudget when adapter advertises it", () => {
    expect(
      liveControlsFor({ caps: baseCaps, thinkingBudgetAdvertised: true }).setThinkingBudget,
    ).toBe(true);
  });

  test("sets setMode when availableModes non-empty", () => {
    const lc = liveControlsFor({
      caps: baseCaps,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      availableModes: [{ modeId: "ask" } as any],
    });
    expect(lc.setMode).toBe(true);
  });

  test("sets mcpServerStatus when http or sse capability advertised", () => {
    const caps = {
      mcpCapabilities: { http: true },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any;
    expect(liveControlsFor({ caps }).mcpServerStatus).toBe(true);
  });

  test("all flags false for bare capabilities", () => {
    const lc = liveControlsFor({ caps: baseCaps });
    expect(lc.setModel).toBe(false);
    expect(lc.setMode).toBe(false);
    expect(lc.setThinkingBudget).toBe(false);
    expect(lc.mcpServerStatus).toBe(false);
    expect(lc.setConfigOption).toBe(true); // stable; always available
  });

  test("handles missing capability fields gracefully", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const caps = {} as any;
    const lc = liveControlsFor({ caps });
    expect(lc.setModel).toBe(false);
    expect(lc.setMode).toBe(false);
    expect(lc.setThinkingBudget).toBe(false);
    expect(lc.mcpServerStatus).toBe(false);
  });
});
