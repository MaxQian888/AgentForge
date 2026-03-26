/** @jest-environment node */
/* eslint-disable @typescript-eslint/no-require-imports */

export {};

function setProcessPlatform(platform: NodeJS.Platform) {
  Object.defineProperty(process, "platform", {
    value: platform,
    configurable: true,
  });
}

describe("dev-workflow command detection", () => {
  const originalPlatform = Object.getOwnPropertyDescriptor(process, "platform");

  afterEach(() => {
    jest.resetModules();
    jest.dontMock("node:child_process");
    jest.dontMock("node:fs");

    if (originalPlatform) {
      Object.defineProperty(process, "platform", originalPlatform);
    }
  });

  test("uses `go version` directly for Go availability", () => {
    const spawnSync = jest.fn(() => ({
      status: 0,
    }));

    jest.doMock("node:child_process", () => ({
      spawnSync,
    }));

    const { isCommandAvailable } = require("./dev-workflow.js");

    expect(isCommandAvailable("go")).toBe(true);
    expect(spawnSync).toHaveBeenCalledWith(
      "go",
      ["version"],
      expect.objectContaining({
        shell: false,
        stdio: "ignore",
      }),
    );
  });

  test("reports Windows Docker Desktop as auto-startable when docker is installed but the daemon is not ready", () => {
    setProcessPlatform("win32");

    const spawnSync = jest.fn((command, args) => {
      if (command === "docker" && args[0] === "--version") {
        return { status: 0 };
      }

      if (command === "docker" && args[0] === "info") {
        return {
          status: 1,
          stdout: "",
          stderr: "daemon not ready",
        };
      }

      return { status: 1, stdout: "", stderr: "" };
    });

    jest.doMock("node:child_process", () => ({
      spawnSync,
      spawn: jest.fn(),
    }));

    jest.doMock("node:fs", () => {
      const actual = jest.requireActual("node:fs");
      return {
        ...actual,
        existsSync: jest.fn((candidate) => {
          if (candidate === "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe") {
            return true;
          }

          return actual.existsSync(candidate);
        }),
      };
    });

    const { getDockerComposeAvailability } = require("./dev-workflow.js");

    const availability = getDockerComposeAvailability();

    expect(availability).toMatchObject({
      ready: false,
      dockerAvailable: true,
      canAutoStart: true,
      reason: "docker_desktop_not_ready",
      dockerDesktopExecutablePath: "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
    });
    expect(spawnSync).toHaveBeenCalledWith(
      "docker",
      ["info"],
      expect.objectContaining({
        shell: false,
        encoding: "utf8",
        timeout: 10000,
      }),
    );
  });

  test("falls back to the Docker Desktop executable when `docker desktop start` cannot launch it", () => {
    setProcessPlatform("win32");

    const unref = jest.fn();
    const spawn = jest.fn(() => ({
      pid: 4242,
      unref,
    }));
    const spawnSync = jest.fn(() => ({
      status: 1,
      stdout: "",
      stderr: "desktop plugin unavailable",
    }));

    jest.doMock("node:child_process", () => ({
      spawn,
      spawnSync,
    }));

    const { startDockerDesktop } = require("./dev-workflow.js");

    const result = startDockerDesktop({
      dockerDesktopExecutablePath: "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
      canAutoStart: true,
    });

    expect(result).toMatchObject({
      ok: true,
      method: "desktop-executable",
      pid: 4242,
    });
    expect(spawnSync).toHaveBeenCalledWith(
      "docker",
      ["desktop", "start"],
      expect.objectContaining({
        shell: false,
        encoding: "utf8",
        timeout: 10000,
      }),
    );
    expect(spawn).toHaveBeenCalledWith(
      "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
      [],
      expect.objectContaining({
        detached: true,
        shell: false,
        stdio: "ignore",
        windowsHide: true,
      }),
    );
    expect(unref).toHaveBeenCalledTimes(1);
  });

  test("does not relaunch Docker Desktop when it is already starting", () => {
    setProcessPlatform("win32");

    const spawn = jest.fn();
    const spawnSync = jest.fn((command, args) => {
      if (command === "docker" && args[0] === "--version") {
        return {
          status: 0,
          stdout: "",
          stderr: "",
        };
      }

      if (command === "docker" && args[0] === "desktop" && args[1] === "status") {
        return {
          status: 0,
          stdout: "Name                Value\r\nStatus              starting\r\n",
          stderr: "",
        };
      }

      return {
        status: 1,
        stdout: "",
        stderr: "",
      };
    });

    jest.doMock("node:child_process", () => ({
      spawn,
      spawnSync,
    }));

    const { startDockerDesktop } = require("./dev-workflow.js");

    const result = startDockerDesktop({
      dockerDesktopExecutablePath: "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
      canAutoStart: true,
    });

    expect(result).toMatchObject({
      ok: true,
      method: "desktop-status-wait",
      desktopStatus: "starting",
    });
    expect(spawnSync).toHaveBeenCalledWith(
      "docker",
      ["desktop", "status"],
      expect.objectContaining({
        shell: false,
        encoding: "utf8",
        timeout: 5000,
      }),
    );
    expect(spawn).not.toHaveBeenCalled();
  });
});
