import { describe, expect, test } from "bun:test";
import { createHttpTransport } from "./http-transport.js";

describe("createHttpTransport", () => {
  test("throws when url is missing", () => {
    expect(() =>
      createHttpTransport({ pluginId: "test", transport: "http" }),
    ).toThrow("missing spec.url");
  });

  test("creates transport with valid url", () => {
    const transport = createHttpTransport({
      pluginId: "test",
      transport: "http",
      url: "http://localhost:3000/mcp",
    });
    expect(transport).toBeDefined();
  });
});
