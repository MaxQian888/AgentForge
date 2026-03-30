import { describe, expect, test } from "bun:test";
import { UnsupportedOperationError } from "./errors.js";

describe("UnsupportedOperationError", () => {
  test("preserves runtime, operation, and the canonical bridge error message", () => {
    const error = new UnsupportedOperationError("getDiff", "opencode");

    expect(error).toBeInstanceOf(Error);
    expect(error.name).toBe("UnsupportedOperationError");
    expect(error.message).toBe("Runtime opencode does not support getDiff");
    expect(error.operation).toBe("getDiff");
    expect(error.runtime).toBe("opencode");
  });
});
