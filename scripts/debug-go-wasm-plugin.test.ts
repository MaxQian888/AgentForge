/** @jest-environment node */

const spawnSyncMock = jest.fn();

jest.mock("node:child_process", () => ({
  spawnSync: (...args: unknown[]) => spawnSyncMock(...args),
}));

jest.mock("./plugin-dev-targets.js", () => ({
  getRepoRoot: () => process.cwd(),
  resolveBuildTarget: ({ manifestPath }: { manifestPath: string }) => ({
    manifestPath,
    pluginId: "feishu-adapter",
    modulePath: `${process.cwd()}\\plugins\\integrations\\feishu-adapter\\dist\\feishu.wasm`,
    sourcePath: "./cmd/sample-wasm-plugin",
  }),
}));

describe("debug-go-wasm-plugin runtime envelope", () => {
  const manifestPath =
    "D:\\Project\\AgentForge\\plugins\\integrations\\feishu-adapter\\manifest.yaml";

  beforeEach(() => {
    jest.resetModules();
    spawnSyncMock.mockReset();
  });

  test("replays health through the Go WASM runtime contract", () => {
    spawnSyncMock.mockReturnValue({
      status: 0,
      stdout: JSON.stringify({
        ok: true,
        operation: "health",
        data: {
          status: "ok",
          mode: "webhook",
        },
      }),
      stderr: "",
    });

    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { runDebugCommand } = require("./debug-go-wasm-plugin.js");

    const result = runDebugCommand({
      manifestPath,
      operation: "health",
    });

    expect(spawnSyncMock).toHaveBeenCalledWith(
      "go",
      [
        "run",
        "./cmd/plugin-debugger",
        "--manifest",
        manifestPath,
        "--operation",
        "health",
      ],
      expect.objectContaining({
        cwd: expect.stringContaining("src-go"),
        encoding: "utf8",
      }),
    );
    expect(result).toMatchObject({
      status: 0,
      output: {
        ok: true,
        operation: "health",
        data: {
          status: "ok",
          mode: "webhook",
        },
      },
      stderr: "",
    });
  });

  test("reports undeclared capability failures with structured output", () => {
    spawnSyncMock.mockReturnValue({
      status: 1,
      stdout: JSON.stringify({
        ok: false,
        operation: "delete_message",
        error: "operation delete_message is not declared in spec.capabilities",
      }),
      stderr: "plugin error",
    });

    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { runDebugCommand } = require("./debug-go-wasm-plugin.js");

    const result = runDebugCommand({
      manifestPath,
      operation: "delete_message",
    });

    expect(result.status).toBe(1);
    expect(result.output).toMatchObject({
      ok: false,
      operation: "delete_message",
    });
    expect(result.output.error).toContain(
      "operation delete_message is not declared in spec.capabilities",
    );
  });
});
