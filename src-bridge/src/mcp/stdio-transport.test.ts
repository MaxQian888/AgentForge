import { describe, expect, test } from "bun:test";
import { interpolateEnv } from "./stdio-transport.js";

describe("interpolateEnv", () => {
  test("replaces ${VAR} with process.env values", () => {
    process.env.TEST_KEY_ABC = "secret123";
    const result = interpolateEnv({ API_KEY: "${TEST_KEY_ABC}" });
    expect(result.API_KEY).toBe("secret123");
    delete process.env.TEST_KEY_ABC;
  });

  test("replaces missing variables with empty string", () => {
    delete process.env.NONEXISTENT_VAR_XYZ;
    const result = interpolateEnv({ VAL: "${NONEXISTENT_VAR_XYZ}" });
    expect(result.VAL).toBe("");
  });

  test("passes through values without interpolation patterns", () => {
    const result = interpolateEnv({ PLAIN: "hello world" });
    expect(result.PLAIN).toBe("hello world");
  });

  test("handles multiple interpolations in one value", () => {
    process.env.PART_A = "foo";
    process.env.PART_B = "bar";
    const result = interpolateEnv({ COMBINED: "${PART_A}:${PART_B}" });
    expect(result.COMBINED).toBe("foo:bar");
    delete process.env.PART_A;
    delete process.env.PART_B;
  });

  test("handles empty env object", () => {
    const result = interpolateEnv({});
    expect(result).toEqual({});
  });
});
