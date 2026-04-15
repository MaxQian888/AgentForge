/** @jest-environment node */

describe("validate-updater-artifacts", () => {
  test("reports manifest entries that point to missing artifact files", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { findMissingManifestArtifacts } = require("./validate-updater-artifacts.js");

    expect(
      findMissingManifestArtifacts({
        availableRelativePaths: new Set([
          "tauri-linux-amd64-appimage/agentforge.AppImage",
        ]),
        manifest: {
          platforms: {
            "linux-x86_64": {
              signature: "linux-signature",
              url:
                "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-linux-amd64-appimage/agentforge.AppImage",
            },
            "windows-x86_64": {
              signature: "windows-signature",
              url:
                "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-windows-x64-nsis/agentforge-setup.exe",
            },
          },
          version: "0.2.0",
        },
      }),
    ).toEqual([
      {
        platform: "windows-x86_64",
        relativePath: "tauri-windows-x64-nsis/agentforge-setup.exe",
        reason: "missing-artifact",
      },
    ]);
  });

  test("reports missing required platform entries", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { findMissingRequiredPlatforms } = require("./validate-updater-artifacts.js");

    expect(
      findMissingRequiredPlatforms({
        manifest: {
          platforms: {
            "linux-x86_64": {
              signature: "linux-signature",
              url:
                "https://github.com/Arxtect/AgentForge/releases/download/v0.2.0/tauri-linux-amd64-appimage/agentforge.AppImage",
            },
          },
          version: "0.2.0",
        },
        requiredPlatforms: ["linux-x86_64", "windows-x86_64", "darwin-aarch64"],
      }),
    ).toEqual([
      {
        platform: "windows-x86_64",
        reason: "missing-platform-entry",
      },
      {
        platform: "darwin-aarch64",
        reason: "missing-platform-entry",
      },
    ]);
  });
});
