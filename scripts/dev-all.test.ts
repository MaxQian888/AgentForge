/** @jest-environment node */
/* eslint-disable @typescript-eslint/no-require-imports */

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

describe("dev-all workflow contract", () => {
  test("builds the full-stack service matrix and repo-local paths", () => {
    const {
      createDevAllServiceDefinitions,
      getDevAllPaths,
    } = require("./dev-all.js");

    const repoRoot = process.cwd();
    const paths = getDevAllPaths({ repoRoot });
    const services = createDevAllServiceDefinitions({ repoRoot });

    expect(paths).toMatchObject({
      repoRoot,
      codexDir: path.join(repoRoot, ".codex"),
      runtimeLogsDir: path.join(repoRoot, ".codex", "runtime-logs"),
      statePath: path.join(repoRoot, ".codex", "dev-all-state.json"),
    });

    expect(
      services.map((service: {
        name: string;
        kind: string;
        start: { source: string };
        port: number;
        healthUrl: string | null;
      }) => ({
        name: service.name,
        kind: service.kind,
        source: service.start.source,
        port: service.port,
        healthUrl: service.healthUrl,
      })),
    ).toEqual([
      {
        name: "postgres",
        kind: "infra",
        source: "docker-compose",
        port: 5432,
        healthUrl: null,
      },
      {
        name: "redis",
        kind: "infra",
        source: "docker-compose",
        port: 6379,
        healthUrl: null,
      },
      {
        name: "go-orchestrator",
        kind: "application",
        source: "spawn",
        port: 7777,
        healthUrl: "http://127.0.0.1:7777/health",
      },
      {
        name: "ts-bridge",
        kind: "application",
        source: "spawn",
        port: 7778,
        healthUrl: "http://127.0.0.1:7778/health",
      },
      {
        name: "frontend",
        kind: "application",
        source: "spawn",
        port: 3000,
        healthUrl: "http://127.0.0.1:3000",
      },
    ]);
  });

  test("reports an untracked but healthy service as external", () => {
    const { reconcileRuntimeState } = require("./dev-workflow.js");

    const report = reconcileRuntimeState({
      serviceDefinitions: [
        {
          name: "frontend",
          port: 3000,
          healthUrl: "http://127.0.0.1:3000",
        },
      ],
      runtimeState: { services: {} },
      liveHealthByService: {
        frontend: true,
      },
      pidExistsByService: {},
    });

    expect(report.services.frontend).toMatchObject({
      name: "frontend",
      source: "external",
      status: "ready",
      health: "healthy",
      managed: false,
    });
  });

  test("marks a tracked managed service as stale when the pid is gone", () => {
    const { reconcileRuntimeState } = require("./dev-workflow.js");

    const report = reconcileRuntimeState({
      serviceDefinitions: [
        {
          name: "frontend",
          port: 3000,
          healthUrl: "http://127.0.0.1:3000",
        },
      ],
      runtimeState: {
        services: {
          frontend: {
            source: "managed",
            pid: 4242,
            healthUrl: "http://127.0.0.1:3000",
            logPath: "D:/Project/AgentForge/.codex/runtime-logs/frontend.log",
            startedAt: "2026-03-25T01:00:00.000Z",
            lastKnownStatus: "ready",
          },
        },
      },
      liveHealthByService: {
        frontend: false,
      },
      pidExistsByService: {
        frontend: false,
      },
    });

    expect(report.services.frontend).toMatchObject({
      source: "managed",
      status: "stale",
      health: "unhealthy",
      managed: true,
      pid: 4242,
    });
  });

  test("creates a stop plan that only targets managed services", () => {
    const { createStopPlan } = require("./dev-workflow.js");

    const plan = createStopPlan({
      runtimeState: {
        services: {
          frontend: {
            source: "managed",
            pid: 111,
          },
          "go-orchestrator": {
            source: "managed",
            pid: 222,
          },
          postgres: {
            source: "reused",
          },
          redis: {
            source: "external",
          },
        },
      },
    });

    expect(plan.toStop).toEqual([
      expect.objectContaining({ name: "frontend", pid: 111 }),
      expect.objectContaining({ name: "go-orchestrator", pid: 222 }),
    ]);
    expect(plan.preserved).toEqual([
      expect.objectContaining({ name: "postgres", source: "reused" }),
      expect.objectContaining({ name: "redis", source: "external" }),
    ]);
  });

  test("exposes the root dev:all command family", () => {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );

    expect(packageJson.scripts).toMatchObject({
      "dev:all": "node scripts/dev-all.js",
      "dev:all:status": "node scripts/dev-all.js status",
      "dev:all:stop": "node scripts/dev-all.js stop",
      "dev:all:logs": "node scripts/dev-all.js logs",
    });
  });

  test("reads known log paths from the persisted runtime state", () => {
    const { runDevAllLogs, getDevAllPaths } = require("./dev-all.js");

    const repoRoot = process.cwd();
    const paths = getDevAllPaths({ repoRoot });
    const originalState = fs.existsSync(paths.statePath)
      ? fs.readFileSync(paths.statePath, "utf8")
      : null;

    fs.mkdirSync(path.dirname(paths.statePath), { recursive: true });
    fs.writeFileSync(
      paths.statePath,
      JSON.stringify(
        {
          version: 1,
          services: {
            frontend: {
              source: "managed",
              logPath: path.join(repoRoot, ".codex", "runtime-logs", "frontend.stdout.log"),
              errorLogPath: path.join(repoRoot, ".codex", "runtime-logs", "frontend.stderr.log"),
            },
          },
        },
        null,
        2,
      ),
      "utf8",
    );

    const result = runDevAllLogs({ repoRoot });

    expect(result.logs).toEqual([
      expect.objectContaining({
        name: "frontend",
        logPath: path.join(repoRoot, ".codex", "runtime-logs", "frontend.stdout.log"),
        errorLogPath: path.join(repoRoot, ".codex", "runtime-logs", "frontend.stderr.log"),
      }),
    ]);

    if (originalState === null) {
      fs.rmSync(paths.statePath, { force: true });
    } else {
      fs.writeFileSync(paths.statePath, originalState, "utf8");
    }
  });

  test("reuses already healthy services instead of spawning duplicates", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-reuse-"));
    const writeRuntimeState = jest.fn();

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        isCommandAvailable: jest.fn(() => true),
        probeServiceHealth: jest.fn(async () => true),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState,
      };
    });

    const { runDevAllStart } = require("./dev-all.js");

    const result = await runDevAllStart({ repoRoot });

    expect(result.ok).toBe(true);
    expect(result.services).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "postgres", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "redis", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "go-orchestrator", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "ts-bridge", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "frontend", action: "reused", source: "reused" }),
      ]),
    );
    expect(writeRuntimeState).toHaveBeenCalledTimes(1);
  });

  test("starts Docker Desktop and continues once docker compose becomes ready", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-docker-start-"));
    const writeRuntimeState = jest.fn();
    const probeCounts = new Map();
    const getDockerComposeAvailability = jest
      .fn()
      .mockReturnValueOnce({
        ready: false,
        dockerAvailable: true,
        canAutoStart: true,
        reason: "docker_desktop_not_ready",
        detail: "Docker Desktop is installed but not ready",
        dockerDesktopExecutablePath: "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
      })
      .mockReturnValueOnce({
        ready: true,
        dockerAvailable: true,
        canAutoStart: true,
        reason: null,
        detail: null,
        dockerDesktopExecutablePath: "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe",
      });
    const startDockerDesktop = jest.fn(() => ({
      ok: true,
      method: "docker-desktop-cli",
    }));
    const runCommandSync = jest.fn(() => ({ status: 0, stdout: "", stderr: "" }));

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        getDockerComposeAvailability,
        startDockerDesktop,
        isCommandAvailable: jest.fn(() => true),
        runCommandSync,
        probeServiceHealth: jest.fn(async (service) => {
          const count = probeCounts.get(service.name) ?? 0;
          probeCounts.set(service.name, count + 1);

          if (service.name === "postgres" || service.name === "redis") {
            return count > 0;
          }

          return true;
        }),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState,
      };
    });

    const { runDevAllStart } = require("./dev-all.js");

    const result = await runDevAllStart({ repoRoot });

    expect(result.ok).toBe(true);
    expect(getDockerComposeAvailability).toHaveBeenCalledTimes(2);
    expect(startDockerDesktop).toHaveBeenCalledTimes(1);
    expect(runCommandSync).toHaveBeenCalledWith(
      "docker",
      ["compose", "up", "-d", "postgres", "redis"],
      expect.any(Object),
    );
  });

  test("rejects unknown listener conflicts before starting a duplicate service", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-conflict-"));
    const writeRuntimeState = jest.fn();

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        isCommandAvailable: jest.fn(() => true),
        probeServiceHealth: jest.fn(async (service) => service.kind === "infra"),
        isPortListening: jest.fn(async (port) => port === 7777),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState,
      };
    });

    const { runDevAllStart, getDevAllPaths } = require("./dev-all.js");

    const result = await runDevAllStart({ repoRoot });

    expect(result.ok).toBe(false);
    expect(result.reason).toBe("external_unknown_listener");
    expect(result.service?.name).toBe("go-orchestrator");
    expect(writeRuntimeState).toHaveBeenCalledWith(
      getDevAllPaths({ repoRoot }).statePath,
      expect.objectContaining({
        services: expect.objectContaining({
          postgres: expect.objectContaining({ source: "reused" }),
          redis: expect.objectContaining({ source: "reused" }),
        }),
      }),
    );
  });

  test("persists partial managed state when startup fails after infra is prepared", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-partial-"));
    const writeRuntimeState = jest.fn();
    const probeCounts = new Map();

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        getDockerComposeAvailability: jest.fn(() => ({
          ready: true,
          dockerAvailable: true,
          canAutoStart: true,
          reason: null,
          detail: null,
        })),
        isCommandAvailable: jest.fn(() => true),
        isPortListening: jest.fn(async (port) => port === 7777),
        runCommandSync: jest.fn(() => ({ status: 0, stdout: "", stderr: "" })),
        probeServiceHealth: jest.fn(async (service) => {
          const key = `${service.name}:${probeCounts.get(service.name) ?? 0}`;
          probeCounts.set(service.name, (probeCounts.get(service.name) ?? 0) + 1);

          if (service.name === "postgres" || service.name === "redis") {
            return key.endsWith(":0") ? false : true;
          }

          return false;
        }),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState,
      };
    });

    const { runDevAllStart, getDevAllPaths } = require("./dev-all.js");

    const result = await runDevAllStart({ repoRoot });

    expect(result.ok).toBe(false);
    expect(result.reason).toBe("external_unknown_listener");
    expect(writeRuntimeState).toHaveBeenCalledWith(
      getDevAllPaths({ repoRoot }).statePath,
      expect.objectContaining({
        services: expect.objectContaining({
          postgres: expect.objectContaining({ source: "managed", composeService: "postgres" }),
          redis: expect.objectContaining({ source: "managed", composeService: "redis" }),
        }),
      }),
    );
  });

  test("stops managed Windows processes and the current listening pid on the service port", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-stop-"));
    const runCommandSync = jest.fn((command, args) => {
      if (command === "cmd.exe" && args[3]?.includes(":3000")) {
        return {
          status: 0,
          stdout: "  TCP    0.0.0.0:3000           0.0.0.0:0              LISTENING       222\r\n",
          stderr: "",
        };
      }

      return { status: 0, stdout: "", stderr: "" };
    });
    const writeRuntimeState = jest.fn();

    const killSpy = jest.spyOn(process, "kill").mockImplementation(() => true);

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        isProcessAlive: jest.fn(() => true),
        runCommandSync,
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {
            frontend: {
              source: "managed",
              pid: 111,
              port: 3000,
            },
            postgres: {
              source: "reused",
              port: 5432,
              composeService: "postgres",
            },
          },
        })),
        writeRuntimeState,
      };
    });

    const { runDevAllStop } = require("./dev-all.js");

    const result = await runDevAllStop({ repoRoot });

    expect(result.ok).toBe(true);
    expect(runCommandSync).toHaveBeenCalledWith(
      "cmd.exe",
      ["/d", "/s", "/c", "netstat -ano | findstr LISTENING | findstr :3000"],
      expect.any(Object),
    );
    expect(killSpy).toHaveBeenCalledWith(111);
    expect(killSpy).toHaveBeenCalledWith(222);
    expect(writeRuntimeState).toHaveBeenCalled();

    killSpy.mockRestore();
  });
});
