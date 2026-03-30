import { describe, expect, test } from "bun:test";

let dynamicImportCounter = 0;

async function importHttpTransportModule() {
  dynamicImportCounter += 1;
  return import(`./http-transport.ts?actual-${dynamicImportCounter}`);
}

describe("createHttpTransport", () => {
  test("throws when url is missing", async () => {
    const { createHttpTransport } = await importHttpTransportModule();
    expect(() =>
      createHttpTransport({ pluginId: "test", transport: "http" }),
    ).toThrow("missing spec.url");
  });

  test("creates transport with valid url", async () => {
    const { createHttpTransport } = await importHttpTransportModule();
    const transport = createHttpTransport({
      pluginId: "test",
      transport: "http",
      url: "http://localhost:3000/mcp",
    });
    expect(transport).toBeDefined();
  });
});
