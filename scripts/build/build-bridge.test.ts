/** @jest-environment node */

import * as fs from "node:fs";
import * as path from "node:path";

describe("build-bridge target resolution", () => {
  test("maps the Windows host triple to the expected Tauri bridge sidecar binary", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveTargets, getOutputFilename } = require("./build-bridge.js");

    expect(
      resolveTargets({
        currentOnly: true,
        hostTriple: "x86_64-pc-windows-msvc",
      }),
    ).toEqual([
      {
        bunTarget: "bun-windows-x64",
        extension: ".exe",
        triple: "x86_64-pc-windows-msvc",
      },
    ]);

    expect(
      getOutputFilename({
        triple: "x86_64-pc-windows-msvc",
        extension: ".exe",
      }),
    ).toBe("bridge-x86_64-pc-windows-msvc.exe");
  });

  test("falls back to the current Windows platform when bun host detection is unavailable", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { getFallbackTriple } = require("./build-bridge.js");

    expect(
      getFallbackTriple({
        platform: "win32",
        arch: "x64",
      }),
    ).toBe("x86_64-pc-windows-msvc");
  });

  test("uses the canonical build script family paths in package commands and docs", () => {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );
    const desktopBuildDoc = fs.readFileSync(
      path.join(process.cwd(), "docs", "deployment", "desktop-build.md"),
      "utf8",
    );

    expect(packageJson.scripts).toMatchObject({
      "build:bridge": "node scripts/build/build-bridge.js",
      "build:bridge:dev": "node scripts/build/build-bridge.js --current-only",
    });
    expect(desktopBuildDoc).toContain("scripts/build/build-bridge.js");
  });
});
