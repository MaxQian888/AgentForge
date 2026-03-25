/** @jest-environment node */

const path = require("node:path");

describe("run-plugin-dev-stack service plan", () => {
  test("builds the minimal Go and Bridge service definitions", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { createServiceDefinitions } = require("./run-plugin-dev-stack.js");

    const services = createServiceDefinitions({
      repoRoot: process.cwd(),
    });

    expect(services).toEqual([
      expect.objectContaining({
        name: "go-orchestrator",
        cwd: path.join(process.cwd(), "src-go"),
        command: "go",
        args: ["run", "./cmd/server"],
        healthUrl: "http://127.0.0.1:7777/health",
      }),
      expect.objectContaining({
        name: "ts-bridge",
        cwd: path.join(process.cwd(), "src-bridge"),
        command: "bun",
        args: ["run", "dev"],
        healthUrl: "http://127.0.0.1:7778/health",
        env: expect.objectContaining({
          PORT: "7778",
          GO_API_URL: "http://127.0.0.1:7777",
          GO_WS_URL: "ws://127.0.0.1:7777/ws/bridge",
        }),
      }),
    ]);
  });

  test("reports missing prerequisites separately from service health", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { collectMissingPrerequisites } = require("./run-plugin-dev-stack.js");

    const missing = collectMissingPrerequisites(
      [
        { name: "go", available: true },
        { name: "bun", available: false },
      ],
    );

    expect(missing).toEqual(["bun"]);
  });
});
