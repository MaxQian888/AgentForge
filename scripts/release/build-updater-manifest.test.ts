/** @jest-environment node */

import * as fs from "node:fs";
import * as path from "node:path";

describe("build-updater-manifest", () => {
  test("builds a Tauri-compatible static manifest from collected updater artifacts", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { buildUpdaterManifest } = require("./build-updater-manifest.js");

    expect(
      buildUpdaterManifest({
        baseDownloadUrl:
          "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0",
        generatedAt: "2026-03-25T04:00:00.000Z",
        releaseVersion: "v0.2.0",
        updaterArtifacts: {
          "darwin-x86_64-app": {
            path: "tauri-macos-x64-updater/AgentForge.app.tar.gz",
            signature: "mac-signature",
          },
          "linux-x86_64-appimage": {
            path: "tauri-linux-amd64-appimage/agentforge.AppImage",
            signature: "linux-signature",
          },
          "windows-x86_64-nsis": {
            path: "tauri-windows-x64-nsis/agentforge-setup.exe",
            signature: "windows-signature",
          },
        },
      }),
    ).toEqual({
      platforms: {
        "darwin-x86_64": {
          signature: "mac-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-macos-x64-updater/AgentForge.app.tar.gz",
        },
        "darwin-x86_64-app": {
          signature: "mac-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-macos-x64-updater/AgentForge.app.tar.gz",
        },
        "linux-x86_64": {
          signature: "linux-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-linux-amd64-appimage/agentforge.AppImage",
        },
        "linux-x86_64-appimage": {
          signature: "linux-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-linux-amd64-appimage/agentforge.AppImage",
        },
        "windows-x86_64": {
          signature: "windows-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-windows-x64-nsis/agentforge-setup.exe",
        },
        "windows-x86_64-nsis": {
          signature: "windows-signature",
          url:
            "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-windows-x64-nsis/agentforge-setup.exe",
        },
      },
      pub_date: "2026-03-25T04:00:00.000Z",
      version: "0.2.0",
    });
  });

  test("keeps release automation and docs aligned with the canonical release script family", () => {
    const releaseWorkflow = fs.readFileSync(
      path.join(process.cwd(), ".github", "workflows", "release.yml"),
      "utf8",
    );
    const updaterDoc = fs.readFileSync(
      path.join(process.cwd(), "docs", "deployment", "desktop-updater-release.md"),
      "utf8",
    );

    expect(releaseWorkflow).toContain("node scripts/release/build-updater-manifest.js");
    expect(releaseWorkflow).toContain("node scripts/release/validate-updater-artifacts.js");
    expect(updaterDoc).toContain("scripts/release/build-updater-manifest.js");
    expect(updaterDoc).toContain("scripts/release/validate-updater-artifacts.js");
  });
});
