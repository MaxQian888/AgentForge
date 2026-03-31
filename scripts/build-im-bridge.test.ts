/** @jest-environment node */

import * as fs from "node:fs";
import * as path from "node:path";

function parseJsonc(filePath: string) {
  return JSON.parse(
    fs.readFileSync(filePath, "utf8").replace(/^\s*\/\/.*$/gmu, ""),
  );
}

describe("build-im-bridge target resolution", () => {
  test("maps the Windows host triple to the expected Tauri IM bridge sidecar binary", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { resolveTargets, getOutputFilename } = require("./build-im-bridge.js");

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

    expect(
      getOutputFilename({
        triple: "x86_64-pc-windows-msvc",
        extension: ".exe",
      }),
    ).toBe("im-bridge-x86_64-pc-windows-msvc.exe");
  });

  test("falls back to the current Windows platform when rustc is unavailable", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { getFallbackTriple } = require("./build-im-bridge.js");

    expect(
      getFallbackTriple({
        platform: "win32",
        arch: "x64",
      }),
    ).toBe("x86_64-pc-windows-msvc");
  });
});

describe("desktop command contract", () => {
  test("exposes IM bridge and shared desktop prepare commands at the root", () => {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );

    expect(packageJson.scripts).toMatchObject({
      "build:im-bridge": "node scripts/build-im-bridge.js",
      "build:im-bridge:dev": "node scripts/build-im-bridge.js --current-only",
      "desktop:dev:prepare":
        "pnpm build:backend:dev && pnpm build:bridge:dev && pnpm build:im-bridge:dev",
      "desktop:build:prepare":
        "pnpm build:backend && pnpm build:bridge && pnpm build:im-bridge && pnpm build",
      "tauri:dev": "pnpm tauri dev",
      "build:desktop": "pnpm tauri build",
    });
  });

  test("wires Tauri pre-commands and external binaries through the shared desktop prepare contract", () => {
    const tauriConfig = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "src-tauri", "tauri.conf.json"), "utf8"),
    );

    expect(tauriConfig.build).toMatchObject({
      beforeDevCommand: "pnpm desktop:dev:prepare && pnpm dev",
      beforeBuildCommand: "pnpm desktop:build:prepare",
    });
    expect(tauriConfig.bundle.externalBin).toEqual([
      "binaries/server",
      "binaries/bridge",
      "binaries/im-bridge",
    ]);
  });

  test("keeps VS Code desktop debug entry points aligned with the shared desktop prepare commands", () => {
    const tasksConfig = parseJsonc(path.join(process.cwd(), ".vscode", "tasks.json"));
    const launchConfig = parseJsonc(path.join(process.cwd(), ".vscode", "launch.json"));

    expect(tasksConfig.tasks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "desktop:dev:prepare",
          command: "pnpm",
          args: ["desktop:dev:prepare"],
        }),
        expect.objectContaining({
          label: "desktop:build:prepare",
          command: "pnpm",
          args: ["desktop:build:prepare"],
        }),
        expect.objectContaining({
          label: "desktop:debug:prepare",
          dependsOn: ["desktop:dev:prepare", "ui:dev-if-needed"],
        }),
      ]),
    );

    expect(launchConfig.configurations).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          name: "Tauri Development Debug",
          preLaunchTask: "desktop:debug:prepare",
        }),
        expect.objectContaining({
          name: "Tauri Production Debug",
          preLaunchTask: "desktop:build:prepare",
        }),
      ]),
    );
  });
});
