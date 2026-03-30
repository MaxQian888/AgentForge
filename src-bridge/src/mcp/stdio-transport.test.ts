import { describe, expect, test } from "bun:test";

let dynamicImportCounter = 0;

async function importStdioTransportModule() {
  dynamicImportCounter += 1;
  return import(`./stdio-transport.ts?actual-${dynamicImportCounter}`);
}

describe("interpolateEnv", () => {
  test("replaces ${VAR} with process.env values", async () => {
    const { interpolateEnv } = await importStdioTransportModule();
    process.env.TEST_KEY_ABC = "secret123";
    const result = interpolateEnv({ API_KEY: "${TEST_KEY_ABC}" });
    expect(result.API_KEY).toBe("secret123");
    delete process.env.TEST_KEY_ABC;
  });

  test("replaces missing variables with empty string", async () => {
    const { interpolateEnv } = await importStdioTransportModule();
    delete process.env.NONEXISTENT_VAR_XYZ;
    const result = interpolateEnv({ VAL: "${NONEXISTENT_VAR_XYZ}" });
    expect(result.VAL).toBe("");
  });

  test("passes through values without interpolation patterns", async () => {
    const { interpolateEnv } = await importStdioTransportModule();
    const result = interpolateEnv({ PLAIN: "hello world" });
    expect(result.PLAIN).toBe("hello world");
  });

  test("handles multiple interpolations in one value", async () => {
    const { interpolateEnv } = await importStdioTransportModule();
    process.env.PART_A = "foo";
    process.env.PART_B = "bar";
    const result = interpolateEnv({ COMBINED: "${PART_A}:${PART_B}" });
    expect(result.COMBINED).toBe("foo:bar");
    delete process.env.PART_A;
    delete process.env.PART_B;
  });

  test("handles empty env object", async () => {
    const { interpolateEnv } = await importStdioTransportModule();
    const result = interpolateEnv({});
    expect(result).toEqual({});
  });
});

describe("createStdioTransport", () => {
  test("throws when the stdio server command is missing", async () => {
    const { createStdioTransport } = await importStdioTransportModule();
    expect(() =>
      createStdioTransport({
        pluginId: "tool.echo",
        transport: "stdio",
      }),
    ).toThrow("MCP server tool.echo is missing spec.command for stdio transport");
  });

  test("merges process env with interpolated plugin env and defaults args to an empty array", async () => {
    const { createStdioTransport } = await importStdioTransportModule();
    process.env.STDIO_BASE_TOKEN = "base-token";
    process.env.STDIO_SHARED_PREFIX = "shared";

    try {
      const transport = createStdioTransport({
        pluginId: "tool.echo",
        transport: "stdio",
        command: "node",
        env: {
          AUTH_TOKEN: "${STDIO_BASE_TOKEN}",
          COMPOSITE: "${STDIO_SHARED_PREFIX}:plugin",
          STATIC_FLAG: "enabled",
        },
      }) as unknown as {
        _serverParams: { command: string; args: string[]; env?: Record<string, string> };
      };

      expect(transport._serverParams.command).toBe("node");
      expect(transport._serverParams.args).toEqual([]);
      expect(transport._serverParams.env).toMatchObject({
        AUTH_TOKEN: "base-token",
        COMPOSITE: "shared:plugin",
        STATIC_FLAG: "enabled",
        STDIO_BASE_TOKEN: "base-token",
      });
    } finally {
      delete process.env.STDIO_BASE_TOKEN;
      delete process.env.STDIO_SHARED_PREFIX;
    }
  });
});
