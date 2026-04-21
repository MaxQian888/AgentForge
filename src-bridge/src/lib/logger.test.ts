import { describe, it, expect } from "bun:test";
import { createLogger, withTrace } from "./logger.js";

describe("logger", () => {
  it("includes trace_id when withTrace is used", () => {
    const captured: string[] = [];
    const base = createLogger({ level: "info", write: (s) => captured.push(s) });
    const child = withTrace(base, "tr_abc");
    child.info({ event: "unit.test" }, "hello");
    expect(captured.length).toBe(1);
    const parsed = JSON.parse(captured[0]);
    expect(parsed.trace_id).toBe("tr_abc");
    expect(parsed.event).toBe("unit.test");
    expect(parsed.msg).toBe("hello");
  });

  it("omits trace_id when none bound", () => {
    const captured: string[] = [];
    const base = createLogger({ level: "info", write: (s) => captured.push(s) });
    base.info({ event: "unit.test" }, "hello");
    const parsed = JSON.parse(captured[0]);
    expect(parsed.trace_id).toBeUndefined();
  });
});
