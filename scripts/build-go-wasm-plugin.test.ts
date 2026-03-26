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
