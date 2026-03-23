/** @jest-environment node */

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
});
