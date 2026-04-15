/** @jest-environment node */

import * as fs from "node:fs";
import * as path from "node:path";

describe("build-backend target resolution", () => {
  test("maps the Windows host triple to the expected Tauri sidecar binary", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveTargets } = require("./build-backend.js");

    expect(
      resolveTargets({
        currentOnly: true,
        hostTriple: "x86_64-pc-windows-msvc",
      }),
    ).toEqual([
      {
        goos: "windows",
        goarch: "amd64",
        triple: "x86_64-pc-windows-msvc",
      },
    ]);
  });

  test("falls back to the current Windows platform when rustc is unavailable", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { getFallbackTriple } = require("./build-backend.js");

    expect(
      getFallbackTriple({
        platform: "win32",
        arch: "x64",
      }),
    ).toBe("x86_64-pc-windows-msvc");
  });

  test("uses the canonical build script family paths in package commands, wrapper, and docs", () => {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );
    const wrapperPath = path.join(process.cwd(), "scripts", "build", "build-backend.sh");
    const desktopBuildDoc = fs.readFileSync(
      path.join(process.cwd(), "docs", "deployment", "desktop-build.md"),
      "utf8",
    );

    expect(packageJson.scripts).toMatchObject({
      "build:backend": "node scripts/build/build-backend.js",
      "build:backend:dev": "node scripts/build/build-backend.js --current-only",
    });
    expect(fs.existsSync(wrapperPath)).toBe(true);
    expect(fs.readFileSync(wrapperPath, "utf8")).toContain(
      'node "$SCRIPT_DIR/build-backend.js" "$@"',
    );
    expect(desktopBuildDoc).toContain("scripts/build/build-backend.js");
  });
});
