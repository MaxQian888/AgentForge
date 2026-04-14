/** @jest-environment node */

import * as path from "node:path";

describe("build-go-wasm-plugin target resolution", () => {
  test("resolves the maintained sample plugin from its manifest path", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveBuildTarget } = require("./build-go-wasm-plugin.js");

    const target = resolveBuildTarget({
      manifestPath: path.join(
        process.cwd(),
        "plugins",
        "integrations",
        "feishu-adapter",
        "manifest.yaml",
      ),
    });

    expect(target).toMatchObject({
      pluginId: "feishu-adapter",
      runtime: "wasm",
      modulePath: path.join(
        process.cwd(),
        "plugins",
        "integrations",
        "feishu-adapter",
        "dist",
        "feishu.wasm",
      ),
      sourcePath: "./cmd/sample-wasm-plugin",
    });
  });

  test("resolves maintained workflow starters from their manifest paths", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveBuildTarget } = require("./build-go-wasm-plugin.js");

    const taskDeliveryTarget = resolveBuildTarget({
      manifestPath: path.join(
        process.cwd(),
        "plugins",
        "workflows",
        "task-delivery-flow",
        "manifest.yaml",
      ),
    });

    expect(taskDeliveryTarget).toMatchObject({
      pluginId: "task-delivery-flow",
      runtime: "wasm",
      modulePath: path.join(
        process.cwd(),
        "plugins",
        "workflows",
        "task-delivery-flow",
        "dist",
        "task-delivery-flow.wasm",
      ),
      sourcePath: "./cmd/task-delivery-flow",
    });

    const reviewEscalationTarget = resolveBuildTarget({
      manifestPath: path.join(
        process.cwd(),
        "plugins",
        "workflows",
        "review-escalation-flow",
        "manifest.yaml",
      ),
    });

    expect(reviewEscalationTarget).toMatchObject({
      pluginId: "review-escalation-flow",
      runtime: "wasm",
      modulePath: path.join(
        process.cwd(),
        "plugins",
        "workflows",
        "review-escalation-flow",
        "dist",
        "review-escalation-flow.wasm",
      ),
      sourcePath: "./cmd/review-escalation-flow",
    });
  });

  test("rejects manifest targets that omit the wasm module path", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveBuildTarget } = require("./build-go-wasm-plugin.js");

    const invalidManifestPath = path.join(
      process.cwd(),
      "scripts",
      "__fixtures__",
      "invalid-go-wasm-plugin-manifest.yaml",
    );

    expect(() =>
      resolveBuildTarget({
        manifestPath: invalidManifestPath,
      }),
    ).toThrow("is missing required field spec.module");
  });
});
