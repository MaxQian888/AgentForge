/** @jest-environment node */

const path = require("node:path");

describe("debug-go-wasm-plugin runtime envelope", () => {
  const manifestPath = path.join(
    process.cwd(),
    "plugins",
    "integrations",
    "feishu-adapter",
    "manifest.yaml",
  );

  beforeAll(() => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { main } = require("./build-go-wasm-plugin.js");
    main(["--manifest", manifestPath]);
  });

  test("replays health through the Go WASM runtime contract", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { runDebugCommand } = require("./debug-go-wasm-plugin.js");

    const result = runDebugCommand({
      manifestPath,
      operation: "health",
    });

    expect(result.status).toBe(0);
    expect(result.output).toMatchObject({
      ok: true,
      operation: "health",
      data: {
        status: "ok",
        mode: "webhook",
      },
    });
    expect(result.stderr).toContain("");
  });

  test("reports undeclared capability failures with structured output", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { runDebugCommand } = require("./debug-go-wasm-plugin.js");

    const result = runDebugCommand({
      manifestPath,
      operation: "delete_message",
    });

    expect(result.status).not.toBe(0);
    expect(result.output).toMatchObject({
      ok: false,
      operation: "delete_message",
    });
    expect(result.output.error).toContain(
      "operation delete_message is not declared in spec.capabilities",
    );
  });
});
