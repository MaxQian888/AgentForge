/** @jest-environment node */
/* eslint-disable @typescript-eslint/no-require-imports */

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

function currentHostTripleForTest() {
  if (process.platform === "win32" && process.arch === "x64") {
    return "x86_64-pc-windows-msvc";
  }

  if (process.platform === "linux" && process.arch === "x64") {
    return "x86_64-unknown-linux-gnu";
  }

  if (process.platform === "linux" && process.arch === "arm64") {
    return "aarch64-unknown-linux-gnu";
  }

  if (process.platform === "darwin" && process.arch === "x64") {
    return "x86_64-apple-darwin";
  }

  if (process.platform === "darwin" && process.arch === "arm64") {
    return "aarch64-apple-darwin";
  }

  return "x86_64-pc-windows-msvc";
}

function hostExecutableExtensionForTest() {
  return process.platform === "win32" ? ".exe" : "";
}

describe("dev-all workflow contract", () => {
  afterEach(() => {
    jest.resetModules();
    jest.dontMock("./dev-workflow.js");
    jest.dontMock("./im-stub-smoke.js");
  });

  test("builds the backend-only service matrix and repo-local paths", () => {
    const {
      createDevBackendServiceDefinitions,
      getDevBackendPaths,
    } = require("./dev-all.js");

    const repoRoot = process.cwd();
    const paths = getDevBackendPaths({ repoRoot });
    const services = createDevBackendServiceDefinitions({ repoRoot });

    expect(paths.repoRoot).toBe(repoRoot);
    expect(paths.statePath).toBe(path.join(paths.codexDir, "dev-backend-state.json"));
    expect(services.find((service: { name: string }) => service.name === "frontend")).toBeUndefined();
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
        name: "im-bridge",
        kind: "application",
        source: "spawn",
        port: 7779,
        healthUrl: "http://127.0.0.1:7779/im/health",
      },
    ]);
  });

  test("builds the full-stack service matrix and repo-local paths", () => {
    const {
      createDevAllServiceDefinitions,
      getDevAllPaths,
    } = require("./dev-all.js");

    const repoRoot = process.cwd();
    const paths = getDevAllPaths({ repoRoot });
    const services = createDevAllServiceDefinitions({ repoRoot });

    expect(paths.repoRoot).toBe(repoRoot);
    expect([
      path.join(repoRoot, ".codex"),
      path.join(repoRoot, "tmp-runtime"),
    ]).toContain(paths.codexDir);
    expect(paths.runtimeLogsDir).toBe(path.join(paths.codexDir, "runtime-logs"));
    expect(paths.statePath).toBe(path.join(paths.codexDir, "dev-all-state.json"));

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
        name: "im-bridge",
        kind: "application",
        source: "spawn",
        port: 7779,
        healthUrl: "http://127.0.0.1:7779/im/health",
      },
      {
        name: "frontend",
        kind: "application",
        source: "spawn",
        port: 3000,
        healthUrl: "http://127.0.0.1:3000",
      },
    ]);

    const imBridge = services.find((service: { name: string }) => service.name === "im-bridge");
    expect(imBridge).toMatchObject({
      cwd: path.join(repoRoot, "src-im-bridge"),
      start: {
        command: "go",
        args: ["run", "./cmd/bridge"],
        env: expect.objectContaining({
          AGENTFORGE_API_BASE: "http://127.0.0.1:7777",
          IM_BRIDGE_ID_FILE: path.join(paths.codexDir, "im-bridge-id"),
          IM_PLATFORM: "feishu",
          IM_TRANSPORT_MODE: "stub",
          NOTIFY_PORT: "7779",
          TEST_PORT: "7780",
        }),
      },
    });
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
      "dev:all": "node scripts/dev/dev-all.js all",
      "dev:all:status": "node scripts/dev/dev-all.js all status",
      "dev:all:stop": "node scripts/dev/dev-all.js all stop",
      "dev:all:logs": "node scripts/dev/dev-all.js all logs",
    });
  });

  test("exposes the root dev:backend command family", () => {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );

    expect(packageJson.scripts).toMatchObject({
      "dev:backend": "node scripts/dev/dev-all.js backend",
      "dev:backend:status": "node scripts/dev/dev-all.js backend status",
      "dev:backend:stop": "node scripts/dev/dev-all.js backend stop",
      "dev:backend:logs": "node scripts/dev/dev-all.js backend logs",
      "dev:backend:verify": "node scripts/dev/dev-all.js backend verify",
    });
  });

  test("runs backend verify with staged smoke output and keeps the managed stack running", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devbackend-verify-"));
    const writeRuntimeState = jest.fn();
    const runIMStubSmoke = jest.fn(async () => ({
      ok: true,
      stages: [
        { name: "stub-command", ok: true, detail: "Injected /agent runtimes" },
        { name: "reply-capture", ok: true, detail: "Captured non-empty reply" },
      ],
      firstReply: { content: "codex: ready" },
      baseUrl: "http://127.0.0.1:7780",
    }));

    jest.doMock("./im-stub-smoke.js", () => ({
      DEFAULT_VERIFY_COMMAND_CONTENT: "/agent runtimes",
      runIMStubSmoke,
    }));

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
        isPortListening: jest.fn(async () => false),
        runCommandSync: jest.fn(() => ({ status: 0, stdout: "", stderr: "" })),
        probeServiceHealth: jest.fn(async () => true),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState,
      };
    });

    const { runDevBackendVerify, getDevBackendPaths } = require("./dev-all.js");

    const result = await runDevBackendVerify({ repoRoot });

    expect(result.ok).toBe(true);
    expect(result.keepRunning).toBe(true);
    expect(result.failureStage).toBeNull();
    expect(result.stages.map((stage: { name: string }) => stage.name)).toEqual([
      "startup",
      "go-health",
      "bridge-health",
      "im-health",
      "stub-command",
      "reply-capture",
      "acp-echo-smoke",
    ]);
    expect(runIMStubSmoke).toHaveBeenCalledWith(
      expect.objectContaining({
        repoRoot,
        platform: "feishu",
        port: 7780,
        commandContent: "/agent runtimes",
      }),
    );
    expect(result.paths).toEqual(getDevBackendPaths({ repoRoot }));
  });

  test("surfaces the failing backend verify stage while preserving runtime diagnostics", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devbackend-verify-fail-"));

    jest.doMock("./im-stub-smoke.js", () => ({
      DEFAULT_VERIFY_COMMAND_CONTENT: "/agent runtimes",
      runIMStubSmoke: jest.fn(async () => ({
        ok: false,
        failureStage: "reply-capture",
        stages: [
          { name: "stub-command", ok: true, detail: "Injected /agent runtimes" },
          { name: "reply-capture", ok: false, detail: "No replies captured" },
        ],
        baseUrl: "http://127.0.0.1:7780",
      })),
    }));

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
        isPortListening: jest.fn(async () => false),
        runCommandSync: jest.fn(() => ({ status: 0, stdout: "", stderr: "" })),
        probeServiceHealth: jest.fn(async () => true),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState: jest.fn(),
      };
    });

    const { runDevBackendVerify } = require("./dev-all.js");

    const result = await runDevBackendVerify({ repoRoot });

    expect(result.ok).toBe(false);
    expect(result.failureStage).toBe("reply-capture");
    expect(result.keepRunning).toBe(true);
    expect(result.stages).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "reply-capture", ok: false, detail: "No replies captured" }),
      ]),
    );
    expect(result.statusReport).toEqual(
      expect.objectContaining({
        ok: true,
        report: expect.objectContaining({
          services: expect.any(Object),
        }),
      }),
    );
  });

  test("backend verify can reuse prepared sidecars when source toolchains are unavailable", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devbackend-verify-sidecars-"));
    const binariesDir = path.join(repoRoot, "src-tauri", "binaries");
    fs.mkdirSync(binariesDir, { recursive: true });
    for (const name of [
      `server-${currentHostTripleForTest()}${hostExecutableExtensionForTest()}`,
      `bridge-${currentHostTripleForTest()}${hostExecutableExtensionForTest()}`,
      `im-bridge-${currentHostTripleForTest()}${hostExecutableExtensionForTest()}`,
    ]) {
      fs.writeFileSync(path.join(binariesDir, name), "", "utf8");
    }

    jest.doMock("./im-stub-smoke.js", () => ({
      DEFAULT_VERIFY_COMMAND_CONTENT: "/agent runtimes",
      runIMStubSmoke: jest.fn(async () => ({
        ok: true,
        stages: [
          { name: "stub-command", ok: true, detail: "Injected /agent runtimes" },
          { name: "reply-capture", ok: true, detail: "Captured non-empty reply" },
        ],
      })),
    }));

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
        isCommandAvailable: jest.fn(() => false),
        isPortListening: jest.fn(async () => false),
        runCommandSync: jest.fn(() => ({ status: 0, stdout: "", stderr: "" })),
        probeServiceHealth: jest.fn(async () => true),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState: jest.fn(),
      };
    });

    const { runDevBackendVerify } = require("./dev-all.js");

    const result = await runDevBackendVerify({ repoRoot });

    expect(result.ok).toBe(true);
  });

  test("reads known log paths from the persisted runtime state", () => {
    const { runDevAllLogs, getDevAllPaths } = require("./dev-all.js");

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-logs-"));
    const paths = getDevAllPaths({ repoRoot });

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

    expect(result).toEqual(expect.objectContaining({ ok: true }));
    expect(result.services).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "postgres", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "redis", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "go-orchestrator", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "ts-bridge", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "im-bridge", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "frontend", action: "reused", source: "reused" }),
      ]),
    );
    expect(writeRuntimeState).toHaveBeenCalledTimes(1);
  });

  test("reuses a healthy backend stack and only starts the frontend for dev:all", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-frontend-only-"));
    const writeRuntimeState = jest.fn();
    const probeCounts = new Map<string, number>();
    const spawn = jest.fn(() => ({
      pid: 5150,
      unref: jest.fn(),
    }));

    jest.doMock("node:child_process", () => {
      const actual = jest.requireActual("node:child_process");
      return {
        ...actual,
        spawn,
      };
    });

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        isCommandAvailable: jest.fn(() => true),
        isPortListening: jest.fn(async () => false),
        probeServiceHealth: jest.fn(async (service) => {
          if (service.name === "frontend") {
            const count = probeCounts.get(service.name) ?? 0;
            probeCounts.set(service.name, count + 1);
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
    expect(result.services).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "go-orchestrator", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "ts-bridge", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "im-bridge", action: "reused", source: "reused" }),
        expect.objectContaining({ name: "frontend", action: "started", source: "managed" }),
      ]),
    );
    expect(spawn).toHaveBeenCalledTimes(1);
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
        isPortListening: jest.fn(async () => false),
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

  test("reports IM bridge listener conflicts through the failing service metadata", async () => {
    jest.resetModules();

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-imbridge-conflict-"));

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        isCommandAvailable: jest.fn(() => true),
        isPortListening: jest.fn(async (port) => port === 7779),
        probeServiceHealth: jest.fn(async (service) => {
          if (service.name === "im-bridge") {
            return false;
          }

          return service.kind === "infra" || service.name === "go-orchestrator" || service.name === "ts-bridge";
        }),
        readRuntimeState: jest.fn(() => ({
          version: 1,
          services: {},
        })),
        writeRuntimeState: jest.fn(),
      };
    });

    const { runDevAllStart } = require("./dev-all.js");

    const result = await runDevAllStart({ repoRoot });

    expect(result.ok).toBe(false);
    expect(result.reason).toBe("external_unknown_listener");
    expect(result.service?.name).toBe("im-bridge");
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
    const originalPlatform = Object.getOwnPropertyDescriptor(process, "platform");
    Object.defineProperty(process, "platform", {
      value: "win32",
      configurable: true,
    });

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-devall-stop-"));
    const killProcessTree = jest.fn(() => true);
    const forceKillProcessTree = jest.fn(() => true);
    const writeRuntimeState = jest.fn();

    jest.doMock("./dev-workflow.js", () => {
      const actual = jest.requireActual("./dev-workflow.js");
      return {
        ...actual,
        canUseDockerCompose: jest.fn(() => true),
        forceKillProcessTree,
        getListeningPidForPort: jest.fn((port) => (port === 3000 ? 222 : null)),
        isPortListening: jest.fn(async (port) => port === 3000),
        isProcessAlive: jest.fn(() => true),
        killProcessTree,
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

    try {
      const result = await runDevAllStop({ repoRoot });

      expect(result.ok).toBe(true);
      expect(killProcessTree).toHaveBeenCalledWith(111);
      expect(forceKillProcessTree).toHaveBeenCalledWith(222);
      expect(writeRuntimeState).toHaveBeenCalled();
    } finally {
      if (originalPlatform) {
        Object.defineProperty(process, "platform", originalPlatform);
      }
    }
  });

  test("shouldRequirePreparedSidecars only fires on Windows without explicit opt-out", () => {
    const { shouldRequirePreparedSidecars } = require("./dev-all.js");

    expect(shouldRequirePreparedSidecars({ platform: "win32", allowSourceServices: undefined })).toBe(true);
    expect(shouldRequirePreparedSidecars({ platform: "win32", allowSourceServices: "1" })).toBe(false);
    expect(shouldRequirePreparedSidecars({ platform: "linux", allowSourceServices: undefined })).toBe(false);
    expect(shouldRequirePreparedSidecars({ platform: "darwin", allowSourceServices: undefined })).toBe(false);
  });

  test("getMissingPreparedSidecars reports application services whose binaries are absent", () => {
    const { createDevBackendServiceDefinitions, getMissingPreparedSidecars } = require("./dev-all.js");

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "dev-all-prep-"));
    try {
      const services = createDevBackendServiceDefinitions({ repoRoot });
      const missing = getMissingPreparedSidecars(services, { repoRoot });
      // Pristine temp dir has no prepared sidecar binaries at all.
      expect(missing.length).toBeGreaterThan(0);
      expect(missing).toEqual(expect.arrayContaining(["go-orchestrator"]));
    } finally {
      fs.rmSync(repoRoot, { recursive: true, force: true });
    }
  });
});
