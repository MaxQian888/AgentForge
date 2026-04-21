#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const crypto = require("node:crypto");
const path = require("node:path");
const { spawn, spawnSync } = require("node:child_process");
const { setTimeout: delay } = require("node:timers/promises");
const { getRepoRoot } = require("../plugin/plugin-dev-targets.js");
const {
  DEFAULT_VERIFY_COMMAND_CONTENT,
  runIMStubSmoke,
} = require("./im-stub-smoke.js");
const { runAcpEchoSmoke } = require("./acp-echo-smoke.js");
const {
  canUseDockerCompose,
  checkPrerequisiteVersions,
  colorBold,
  colorCyan,
  colorDim,
  colorGreen,
  colorRed,
  colorYellow,
  createEmptyRuntimeState,
  createStopPlan,
  ensureDirectory,
  forceKillProcessTree,
  getDockerComposeAvailability,
  getListeningPidForPort,
  getPortOwnerInfo,
  getWorkflowPaths,
  isCommandAvailable,
  isPortListening,
  isProcessAlive,
  killProcessTree,
  probeServiceHealth,
  readRuntimeState,
  reconcileRuntimeState,
  runCommandSync,
  startDockerDesktop,
  statusIcon,
  statusLabel,
  tailFile,
  writeRuntimeState,
} = require("./dev-workflow.js");

const WORKFLOW_PROFILES = {
  all: {
    profile: "all",
    commandLabel: "dev-all",
    displayName: "Full-stack local development",
    stateFileName: "dev-all-state.json",
    serviceNames: [
      "postgres",
      "redis",
      "go-orchestrator",
      "ts-bridge",
      "im-bridge",
      "frontend",
    ],
  },
  backend: {
    profile: "backend",
    commandLabel: "dev-backend",
    displayName: "Backend-only local development",
    stateFileName: "dev-backend-state.json",
    serviceNames: [
      "postgres",
      "redis",
      "go-orchestrator",
      "ts-bridge",
      "im-bridge",
    ],
  },
};

const LOCAL_DEV_JWT_SECRET =
  process.env.JWT_SECRET ?? "agentforge-local-dev-jwt-secret-for-runtime-smoke";

function currentHostTriple() {
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

function executableExtension() {
  return process.platform === "win32" ? ".exe" : "";
}

function getPreparedSidecarBinaryName(serviceName) {
  if (serviceName === "go-orchestrator") {
    return "server";
  }

  if (serviceName === "ts-bridge") {
    return "bridge";
  }

  if (serviceName === "im-bridge") {
    return "im-bridge";
  }

  return null;
}

function getPreparedSidecarBinaryPath({ repoRoot = getRepoRoot(), serviceName } = {}) {
  const binaryName = getPreparedSidecarBinaryName(serviceName);
  if (!binaryName) {
    return null;
  }

  const candidate = path.join(
    repoRoot,
    "src-tauri",
    "binaries",
    `${binaryName}-${currentHostTriple()}${executableExtension()}`,
  );

  return fs.existsSync(candidate) ? candidate : null;
}

function shouldRequirePreparedSidecars({
  platform = process.platform,
  allowSourceServices = process.env.AGENTFORGE_DEV_ALLOW_SOURCE_SERVICES,
} = {}) {
  // Source-mode `go run` startup is unreliable on Windows (antivirus, long
  // paths, pnpm filter fan-out). Require pre-built sidecars unless the
  // operator explicitly opts out.
  return platform === "win32" && allowSourceServices !== "1";
}

function getMissingPreparedSidecars(serviceDefinitions, { repoRoot = getRepoRoot() } = {}) {
  return serviceDefinitions
    .filter((service) => service.kind === "application")
    .filter((service) => getPreparedSidecarBinaryName(service.name))
    .filter((service) => !getPreparedSidecarBinaryPath({ repoRoot, serviceName: service.name }))
    .map((service) => service.name);
}

function runDesktopDevPrepare({ repoRoot = getRepoRoot(), progress = () => {} } = {}) {
  const useCmd = process.platform === "win32";
  const command = useCmd ? "cmd.exe" : "pnpm";
  const args = useCmd
    ? ["/d", "/s", "/c", "pnpm desktop:dev:prepare"]
    : ["desktop:dev:prepare"];

  progress(`preparing sidecars via \`${useCmd ? "cmd.exe /d /s /c " : ""}pnpm desktop:dev:prepare\``);
  progress("building Go orchestrator, TS bridge, and IM bridge once before startup (this can take ~1 min)");

  const result = spawnSync(command, args, {
    cwd: repoRoot,
    env: process.env,
    stdio: "inherit",
    windowsHide: true,
  });

  if (result.status !== 0) {
    return {
      ok: false,
      detail:
        result.error?.message ||
        `pnpm desktop:dev:prepare failed with exit code ${result.status ?? "unknown"}`,
    };
  }

  progress("prepared sidecar build completed");
  return { ok: true };
}

function applyPreparedSidecarOverrides(service, { repoRoot = getRepoRoot(), preferPreparedSidecars = false } = {}) {
  if (!preferPreparedSidecars || service.kind !== "application") {
    return service;
  }

  const preparedBinary = getPreparedSidecarBinaryPath({
    repoRoot,
    serviceName: service.name,
  });

  if (!preparedBinary) {
    return service;
  }

  return {
    ...service,
    cwd: repoRoot,
    start: {
      ...service.start,
      command: preparedBinary,
      args: [],
      preparedBinary,
    },
  };
}

function getWorkflowProfile(profile = "all") {
  return WORKFLOW_PROFILES[profile] ?? null;
}

function getWorkflowPathsForProfile({ profile = "all", repoRoot = getRepoRoot() } = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  return getWorkflowPaths({
    repoRoot,
    stateFileName: workflowProfile.stateFileName,
  });
}

function getDevAllPaths({ repoRoot = getRepoRoot() } = {}) {
  return getWorkflowPathsForProfile({
    profile: "all",
    repoRoot,
  });
}

function getDevBackendPaths({ repoRoot = getRepoRoot() } = {}) {
  return getWorkflowPathsForProfile({
    profile: "backend",
    repoRoot,
  });
}

function detectAirAvailable() {
  return isCommandAvailable("air", ["--version"]);
}

function getGoOrchestratorStartConfig({ repoRoot, jwtSecret, useAir = false }) {
  const goEnv = {
    ENV: "development",
    PORT: "7777",
    JWT_SECRET: jwtSecret,
    GOCACHE: path.join(repoRoot, "src-go", ".gocache"),
    GOFLAGS: "-p=1",
    POSTGRES_URL: "postgres://dev:dev@127.0.0.1:5432/appdb?sslmode=disable",
    REDIS_URL: "redis://127.0.0.1:6379",
    BRIDGE_URL: "http://127.0.0.1:7778",
  };

  if (useAir) {
    return {
      source: "spawn",
      command: "air",
      args: [],
      env: goEnv,
      hotReload: true,
    };
  }

  return {
    source: "spawn",
    command: "go",
    args: ["run", "./cmd/server"],
    env: goEnv,
    hotReload: false,
  };
}

function createServiceDefinitionsForProfile({
  profile = "all",
  repoRoot = getRepoRoot(),
  preferPreparedSidecars = false,
} = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  const workflowPaths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const jwtSecret = LOCAL_DEV_JWT_SECRET;
  const imBridgeAccessToken =
    process.env.AGENTFORGE_API_KEY ?? createLocalDevAccessToken({ secret: jwtSecret });

  const preferAir = process.env.PREFER_AIR === "1" || process.env.PREFER_AIR === "true";
  const useAir = preferAir && detectAirAvailable();

  return [
    {
      name: "postgres",
      kind: "infra",
      composeService: "postgres",
      port: 5432,
      healthUrl: null,
      start: {
        source: "docker-compose",
      },
    },
    {
      name: "redis",
      kind: "infra",
      composeService: "redis",
      port: 6379,
      healthUrl: null,
      start: {
        source: "docker-compose",
      },
    },
    {
      name: "go-orchestrator",
      kind: "application",
      cwd: path.join(repoRoot, "src-go"),
      port: 7777,
      healthUrl: "http://127.0.0.1:7777/health",
      start: getGoOrchestratorStartConfig({ repoRoot, jwtSecret, useAir }),
    },
    {
      name: "ts-bridge",
      kind: "application",
      cwd: path.join(repoRoot, "src-bridge"),
      port: 7778,
      healthUrl: "http://127.0.0.1:7778/health",
      start: {
        source: "spawn",
        command: "bun",
        args: ["run", "dev"],
        env: {
          PORT: "7778",
          GO_API_URL: "http://127.0.0.1:7777",
          GO_WS_URL: "ws://127.0.0.1:7777/ws/bridge",
        },
      },
    },
    {
      name: "im-bridge",
      kind: "application",
      cwd: path.join(repoRoot, "src-im-bridge"),
      port: 7779,
      healthUrl: "http://127.0.0.1:7779/im/health",
      start: {
        source: "spawn",
        command: "go",
        args: ["run", "./cmd/bridge"],
        env: {
          AGENTFORGE_API_BASE: "http://127.0.0.1:7777",
          AGENTFORGE_API_KEY: imBridgeAccessToken,
          AGENTFORGE_PROJECT_ID: process.env.AGENTFORGE_PROJECT_ID ?? "",
          IM_BRIDGE_ID_FILE: path.join(workflowPaths.codexDir, "im-bridge-id"),
          IM_PLATFORM: process.env.IM_PLATFORM ?? "feishu",
          IM_TRANSPORT_MODE: process.env.IM_TRANSPORT_MODE ?? "stub",
          FEISHU_APP_ID: process.env.FEISHU_APP_ID ?? "",
          FEISHU_APP_SECRET: process.env.FEISHU_APP_SECRET ?? "",
          NOTIFY_PORT: "7779",
          TEST_PORT: "7780",
        },
      },
    },
    {
      name: "frontend",
      kind: "application",
      cwd: repoRoot,
      port: 3000,
      healthUrl: "http://127.0.0.1:3000",
      start: {
        source: "spawn",
        command: "pnpm",
        args: ["dev"],
        env: {
          NEXT_PUBLIC_API_URL: "http://127.0.0.1:7777",
        },
      },
    },
  ]
    .filter((service) => workflowProfile.serviceNames.includes(service.name))
    .map((service) =>
      applyPreparedSidecarOverrides(service, {
        repoRoot,
        preferPreparedSidecars,
      }),
    );
}

function createDevAllServiceDefinitions({ repoRoot = getRepoRoot() } = {}) {
  return createServiceDefinitionsForProfile({
    profile: "all",
    repoRoot,
  });
}

function createDevBackendServiceDefinitions({ repoRoot = getRepoRoot() } = {}) {
  return createServiceDefinitionsForProfile({
    profile: "backend",
    repoRoot,
  });
}

function getApplicationServices(serviceDefinitions) {
  return serviceDefinitions.filter((service) => service.kind === "application");
}

function getInfrastructureServices(serviceDefinitions) {
  return serviceDefinitions.filter((service) => service.kind === "infra");
}

function getServiceLogPaths(paths, serviceName) {
  return {
    stdoutPath: path.join(paths.runtimeLogsDir, `${serviceName}.stdout.log`),
    stderrPath: path.join(paths.runtimeLogsDir, `${serviceName}.stderr.log`),
  };
}

function getCommandAvailabilityCheck(service) {
  if (!service.start?.command) {
    return null;
  }

  if (fs.existsSync(service.start.command)) {
    return {
      serviceName: service.name,
      command: service.start.command,
      available: true,
    };
  }

  return {
    serviceName: service.name,
    command: service.start.command,
    available: isCommandAvailable(service.start.command),
  };
}

function getSpawnCommand(service) {
  if (process.platform === "win32" && service.start.command === "pnpm") {
    return {
      command: "cmd.exe",
      args: ["/d", "/s", "/c", [service.start.command, ...service.start.args].join(" ")],
    };
  }

  return {
    command: service.start.command,
    args: service.start.args,
  };
}

async function waitForServiceHealth(service, timeoutMs = 30000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await probeServiceHealth(service)) {
      return true;
    }
    await delay(1000);
  }

  return false;
}

function createManagedServiceRecord(service, logPaths, pid) {
  return {
    source: "managed",
    pid,
    port: service.port,
    healthUrl: service.healthUrl,
    logPath: logPaths.stdoutPath,
    errorLogPath: logPaths.stderrPath,
    composeService: service.composeService ?? null,
    startedAt: new Date().toISOString(),
    lastKnownStatus: "ready",
  };
}

async function ensureApplicationService(service, paths, runtimeState) {
  const trackedState = runtimeState.services?.[service.name];

  if (await probeServiceHealth(service)) {
    return {
      ok: true,
      action: "reused",
      service,
      record: {
        ...(trackedState ?? {}),
        source: trackedState?.source === "managed" ? "managed" : "reused",
        port: service.port,
        healthUrl: service.healthUrl,
        composeService: null,
        lastKnownStatus: "ready",
      },
    };
  }

  if (await isPortListening(service.port)) {
    return {
      ok: false,
      action: "conflict",
      service,
      reason: "external_unknown_listener",
      detail: `Port ${service.port} is occupied but ${service.name} did not pass health check`,
    };
  }

  const logPaths = getServiceLogPaths(paths, service.name);
  ensureDirectory(paths.runtimeLogsDir);
  const stdoutFd = fs.openSync(logPaths.stdoutPath, "a");
  const stderrFd = fs.openSync(logPaths.stderrPath, "a");

  const spawnCommand = getSpawnCommand(service);
  const child = spawn(spawnCommand.command, spawnCommand.args, {
    cwd: service.cwd,
    detached: true,
    shell: false,
    stdio: ["ignore", stdoutFd, stderrFd],
    env: {
      ...process.env,
      ...service.start.env,
    },
  });

  child.unref();

  if (!(await waitForServiceHealth(service))) {
    return {
      ok: false,
      action: "unhealthy",
      service,
      reason: "startup_failed",
      detail: `${service.name} did not become healthy at ${service.healthUrl}`,
      logPaths,
      pid: child.pid,
    };
  }

  return {
    ok: true,
    action: "started",
    service,
    record: createManagedServiceRecord(service, logPaths, child.pid),
  };
}

function isInfraOptional(serviceName) {
  // Redis is optional — Go backend runs in degraded mode without it
  return serviceName === "redis";
}

function getInfraInstallHint(serviceName) {
  const hints = {
    postgres: "Install PostgreSQL locally or start Docker Desktop. Native install: https://www.postgresql.org/download/",
    redis: "Install Redis locally or start Docker Desktop. Native install: https://redis.io/download/",
  };
  return hints[serviceName] ?? `Install ${serviceName} or start Docker Desktop`;
}

async function ensureInfrastructure(repoRoot, services, runtimeState) {
  const results = [];
  const missingInfra = [];

  for (const service of services) {
    const trackedState = runtimeState.services?.[service.name];

    // Probe 1: check if service is already healthy (native install, external, or previous run)
    if (await probeServiceHealth(service)) {
      results.push({
        ok: true,
        action: "reused",
        service,
        record: {
          ...(trackedState ?? {}),
          source: trackedState?.source === "managed" ? "managed" : "reused",
          port: service.port,
          healthUrl: null,
          composeService: service.composeService,
          lastKnownStatus: "ready",
        },
      });
      continue;
    }

    // Probe 2: port is occupied by something else
    if (await isPortListening(service.port)) {
      const ownerInfo = getPortOwnerInfo(service.port);
      const ownerDetail = ownerInfo
        ? ` (PID ${ownerInfo.pid}, ${ownerInfo.processName})`
        : "";
      return {
        ok: false,
        reason: "external_unknown_listener",
        detail: `Port ${service.port} is occupied${ownerDetail} but ${service.name} is not responding as expected`,
        service,
        results,
      };
    }

    missingInfra.push(service);
  }

  if (missingInfra.length === 0) {
    return { ok: true, results };
  }

  // Try docker-compose for missing infra
  const dockerComposeReady = await ensureDockerComposeReady();
  if (!dockerComposeReady.ok) {
    // Docker unavailable — check which services are optional vs required
    const requiredMissing = missingInfra.filter((s) => !isInfraOptional(s.name));
    const optionalMissing = missingInfra.filter((s) => isInfraOptional(s.name));

    // Mark optional services as degraded
    for (const service of optionalMissing) {
      results.push({
        ok: true,
        action: "skipped",
        service,
        record: {
          source: "unavailable",
          pid: null,
          port: service.port,
          healthUrl: null,
          composeService: service.composeService,
          logPath: null,
          errorLogPath: null,
          startedAt: new Date().toISOString(),
          lastKnownStatus: "degraded",
        },
      });
    }

    if (requiredMissing.length > 0) {
      const hints = requiredMissing.map((s) => `  - ${s.name}: ${getInfraInstallHint(s.name)}`).join("\n");
      return {
        ok: false,
        reason: "infra_unavailable",
        detail: `Required infrastructure not running and Docker is unavailable:\n${hints}`,
        service: requiredMissing[0],
        results,
      };
    }

    // All missing infra was optional
    return { ok: true, results };
  }

  const composeServices = missingInfra.map((service) => service.composeService);
  const composeResult = runCommandSync("docker", ["compose", "up", "-d", ...composeServices], {
    cwd: repoRoot,
    encoding: "utf8",
  });

  if (composeResult.status !== 0) {
    return {
      ok: false,
      reason: "docker_compose_failed",
      detail: composeResult.stderr?.trim() || composeResult.stdout?.trim() || "docker compose up failed",
      service: missingInfra[0],
      results,
    };
  }

  for (const service of missingInfra) {
    if (!(await waitForServiceHealth(service))) {
      if (isInfraOptional(service.name)) {
        results.push({
          ok: true,
          action: "skipped",
          service,
          record: {
            source: "unavailable",
            pid: null,
            port: service.port,
            healthUrl: null,
            composeService: service.composeService,
            logPath: null,
            errorLogPath: null,
            startedAt: new Date().toISOString(),
            lastKnownStatus: "degraded",
          },
        });
        continue;
      }

      return {
        ok: false,
        reason: "infra_unhealthy",
        detail: `${service.name} did not become reachable on port ${service.port}`,
        service,
        results,
      };
    }

    results.push({
      ok: true,
      action: "started",
      service,
      record: {
        source: "managed",
        pid: null,
        port: service.port,
        healthUrl: null,
        composeService: service.composeService,
        logPath: null,
        errorLogPath: null,
        startedAt: new Date().toISOString(),
        lastKnownStatus: "ready",
      },
    });
  }

  return { ok: true, results };
}

async function ensureDockerComposeReady(timeoutMs = 180000) {
  const availability = getDockerComposeAvailability();
  if (availability.ready) {
    return {
      ok: true,
      availability,
    };
  }

  if (!availability.canAutoStart) {
    return {
      ok: false,
      reason: "missing_prerequisite",
      detail: availability.detail ?? "docker compose is unavailable or Docker Desktop is not ready",
      availability,
    };
  }

  const startResult = startDockerDesktop(availability);
  if (!startResult.ok) {
    return {
      ok: false,
      reason: startResult.reason ?? "missing_prerequisite",
      detail: startResult.detail ?? availability.detail,
      availability,
    };
  }

  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const nextAvailability = getDockerComposeAvailability();
    if (nextAvailability.ready) {
      return {
        ok: true,
        availability: nextAvailability,
        startResult,
      };
    }

    await delay(2000);
  }

  const finalAvailability = getDockerComposeAvailability();
  return {
    ok: false,
    reason: "missing_prerequisite",
    detail:
      finalAvailability.detail ??
      "Docker Desktop did not become ready before the startup timeout elapsed",
    availability: finalAvailability,
    startResult,
  };
}

function buildRuntimeStateFromResults(previousState, results) {
  const nextState = {
    ...createEmptyRuntimeState(),
    ...previousState,
    services: {
      ...(previousState?.services ?? {}),
    },
  };

  for (const result of results) {
    nextState.services[result.service.name] = result.record;
  }

  return nextState;
}

function createFailureRecord(result) {
  if (!result?.service) {
    return null;
  }

  if (result.pid || result.logPaths?.stdoutPath || result.logPaths?.stderrPath) {
    return {
      service: result.service,
      record: {
        source: "managed",
        pid: result.pid ?? null,
        port: result.service.port ?? null,
        healthUrl: result.service.healthUrl ?? null,
        composeService: result.service.composeService ?? null,
        logPath: result.logPaths?.stdoutPath ?? null,
        errorLogPath: result.logPaths?.stderrPath ?? null,
        startedAt: new Date().toISOString(),
        lastKnownStatus: result.reason ?? "startup_failed",
      },
    };
  }

  return null;
}

function persistPartialState(paths, runtimeState, successfulResults, failingResult = null) {
  const resultsToPersist = [...successfulResults];
  const failureRecord = createFailureRecord(failingResult);
  if (failureRecord) {
    resultsToPersist.push(failureRecord);
  }

  if (resultsToPersist.length === 0) {
    return;
  }

  writeRuntimeState(paths.statePath, buildRuntimeStateFromResults(runtimeState, resultsToPersist));
}

async function runWorkflowStart({
  profile = "all",
  repoRoot = getRepoRoot(),
  preferPreparedSidecars = false,
} = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  ensureDirectory(paths.codexDir);
  ensureDirectory(paths.runtimeLogsDir);

  const runtimeState = readRuntimeState(paths.statePath);
  const serviceDefinitions = createServiceDefinitionsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
    preferPreparedSidecars,
  });
  const infrastructureServices = getInfrastructureServices(serviceDefinitions);
  const applicationServices = getApplicationServices(serviceDefinitions);

  // Print prerequisite versions for visibility
  printPrerequisiteVersions(applicationServices);

  const applicationChecks = applicationServices
    .map(getCommandAvailabilityCheck)
    .filter(Boolean);
  const missingCommands = applicationChecks.filter((check) => !check.available);
  if (missingCommands.length > 0) {
    return {
      ok: false,
      reason: "missing_prerequisite",
      detail: `Missing prerequisites: ${missingCommands.map((check) => check.command).join(", ")}`,
      service: missingCommands[0]?.serviceName ?? null,
    };
  }

  const allResults = [];
  const infraResult = await ensureInfrastructure(repoRoot, infrastructureServices, runtimeState);
  if (!infraResult.ok) {
    persistPartialState(paths, runtimeState, infraResult.results ?? [], infraResult);
    return infraResult;
  }
  allResults.push(...infraResult.results);

  for (const service of applicationServices) {
    const result = await ensureApplicationService(service, paths, runtimeState);
    if (!result.ok) {
      persistPartialState(paths, runtimeState, allResults, result);
      return result;
    }
    allResults.push(result);
  }

  const nextState = buildRuntimeStateFromResults(runtimeState, allResults);
  writeRuntimeState(paths.statePath, nextState);

  return {
    ok: true,
    status: "ready",
    paths,
    services: allResults.map((result) => ({
      name: result.service.name,
      action: result.action,
      source: result.record.source,
      port: result.record.port ?? null,
      healthUrl: result.record.healthUrl ?? null,
      logPath: result.record.logPath ?? null,
      errorLogPath: result.record.errorLogPath ?? null,
      hotReload: result.service.start?.hotReload ?? false,
    })),
  };
}

function base64UrlEncode(value) {
  return Buffer.from(value).toString("base64url");
}

function createLocalDevAccessToken({
  secret = LOCAL_DEV_JWT_SECRET,
  userId = "im-bridge-local",
  email = "im-bridge@agentforge.local",
  ttlSeconds = 24 * 60 * 60,
  now = Math.floor(Date.now() / 1000),
} = {}) {
  const header = {
    alg: "HS256",
    typ: "JWT",
  };
  const payload = {
    user_id: userId,
    email,
    jti: crypto.randomUUID(),
    sub: userId,
    iat: now,
    exp: now + ttlSeconds,
  };
  const encodedHeader = base64UrlEncode(JSON.stringify(header));
  const encodedPayload = base64UrlEncode(JSON.stringify(payload));
  const signature = crypto
    .createHmac("sha256", secret)
    .update(`${encodedHeader}.${encodedPayload}`)
    .digest("base64url");
  return `${encodedHeader}.${encodedPayload}.${signature}`;
}

async function runDevAllStart({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStart({
    profile: "all",
    repoRoot,
  });
}

async function runDevBackendStart({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStart({
    profile: "backend",
    repoRoot,
  });
}

function createVerifyStage(name, ok, detail, extras = {}) {
  return {
    name,
    ok,
    detail,
    ...extras,
  };
}

function getServiceResultByName(startResult, serviceName) {
  return startResult.services.find((service) => service.name === serviceName) ?? null;
}

async function runWorkflowVerify({ profile = "backend", repoRoot = getRepoRoot() } = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.backend;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const stages = [];
  const startResult = await runWorkflowStart({
    profile: workflowProfile.profile,
    repoRoot,
    preferPreparedSidecars: true,
  });

  if (!startResult.ok) {
    stages.push(
      createVerifyStage(
        "startup",
        false,
        startResult.detail ?? `${workflowProfile.displayName} startup failed`,
        {
          service: startResult.service?.name ?? null,
          paths,
        },
      ),
    );
    return {
      ok: false,
      status: "startup_failed",
      keepRunning: true,
      failureStage: "startup",
      paths,
      startResult,
      stages,
      statusReport: await runWorkflowStatus({
        profile: workflowProfile.profile,
        repoRoot,
      }),
    };
  }

  stages.push(
    createVerifyStage("startup", true, `${workflowProfile.displayName} ready`, {
      paths,
      services: startResult.services,
    }),
  );

  const serviceDefinitions = createServiceDefinitionsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const healthChecks = [
    { name: "go-health", serviceName: "go-orchestrator" },
    { name: "bridge-health", serviceName: "ts-bridge" },
    { name: "im-health", serviceName: "im-bridge" },
  ];

  for (const check of healthChecks) {
    const service = serviceDefinitions.find((candidate) => candidate.name === check.serviceName);
    const serviceResult = getServiceResultByName(startResult, check.serviceName);
    if (!service) {
      stages.push(
        createVerifyStage(check.name, false, `Missing service definition for ${check.serviceName}`, {
          paths,
        }),
      );
      return {
        ok: false,
        status: "verify_failed",
        keepRunning: true,
        failureStage: check.name,
        paths,
        startResult,
        stages,
        statusReport: await runWorkflowStatus({
          profile: workflowProfile.profile,
          repoRoot,
        }),
      };
    }

    const healthy = await probeServiceHealth(service);
    if (!healthy) {
      stages.push(
        createVerifyStage(check.name, false, `Health check failed for ${service.name}`, {
          endpoint: service.healthUrl,
          logPath: serviceResult?.logPath ?? null,
          errorLogPath: serviceResult?.errorLogPath ?? null,
          paths,
        }),
      );
      return {
        ok: false,
        status: "verify_failed",
        keepRunning: true,
        failureStage: check.name,
        paths,
        startResult,
        stages,
        statusReport: await runWorkflowStatus({
          profile: workflowProfile.profile,
          repoRoot,
        }),
      };
    }

    stages.push(
      createVerifyStage(check.name, true, `Health check passed for ${service.name}`, {
        endpoint: service.healthUrl,
        logPath: serviceResult?.logPath ?? null,
        errorLogPath: serviceResult?.errorLogPath ?? null,
      }),
    );
  }

  const imBridgeService = serviceDefinitions.find((service) => service.name === "im-bridge");
  const smokeResult = await runIMStubSmoke({
    repoRoot,
    platform: imBridgeService?.start?.env?.IM_PLATFORM ?? "feishu",
    port: Number(imBridgeService?.start?.env?.TEST_PORT ?? 7780),
    commandContent: DEFAULT_VERIFY_COMMAND_CONTENT,
  });
  stages.push(...smokeResult.stages);

  const statusReport = await runWorkflowStatus({
    profile: workflowProfile.profile,
    repoRoot,
  });

  if (!smokeResult.ok) {
    return {
      ok: false,
      status: "verify_failed",
      keepRunning: true,
      failureStage: smokeResult.failureStage,
      paths,
      startResult,
      smokeResult,
      stages,
      statusReport,
    };
  }

  // ACP echo smoke — gated by VERIFY_ACP=1 (default OFF). Requires real agent
  // CLIs / API keys. Failures are logged per-adapter but do not abort other
  // adapters; the overall verify result is only failed if ACP smoke is enabled
  // (VERIFY_ACP=1) and at least one adapter fails.
  const acpSmokeResult = await runAcpEchoSmoke();
  stages.push(...acpSmokeResult.stages);
  if (!acpSmokeResult.ok && !acpSmokeResult.skipped) {
    return {
      ok: false,
      status: "verify_failed",
      keepRunning: true,
      failureStage: "acp-echo-smoke",
      paths,
      startResult,
      smokeResult,
      acpSmokeResult,
      stages,
      statusReport,
    };
  }

  return {
    ok: true,
    status: "verified",
    keepRunning: true,
    failureStage: null,
    paths,
    startResult,
    smokeResult,
    acpSmokeResult,
    stages,
    statusReport,
  };
}

async function runDevBackendVerify({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowVerify({
    profile: "backend",
    repoRoot,
  });
}

async function collectLiveHealth(serviceDefinitions) {
  const liveHealthByService = {};
  const pidExistsByService = {};

  for (const service of serviceDefinitions) {
    liveHealthByService[service.name] = await probeServiceHealth(service);
  }

  return { liveHealthByService, pidExistsByService };
}

async function runWorkflowStatus({ profile = "all", repoRoot = getRepoRoot() } = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const runtimeState = readRuntimeState(paths.statePath);
  const serviceDefinitions = createServiceDefinitionsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const { liveHealthByService, pidExistsByService } = await collectLiveHealth(serviceDefinitions);

  for (const service of serviceDefinitions) {
    const trackedState = runtimeState.services?.[service.name];
    if (trackedState?.pid) {
      pidExistsByService[service.name] = isProcessAlive(trackedState.pid);
    }
  }

  const report = reconcileRuntimeState({
    serviceDefinitions,
    runtimeState,
    liveHealthByService,
    pidExistsByService,
  });

  const nextState = {
    ...runtimeState,
    services: {
      ...(runtimeState.services ?? {}),
    },
  };

  for (const service of Object.values(report.services)) {
    const tracked = nextState.services[service.name];
    if (!tracked) {
      continue;
    }

    nextState.services[service.name] = {
      ...tracked,
      lastKnownStatus: service.status,
    };
  }

  writeRuntimeState(paths.statePath, nextState);

  return {
    ok: true,
    paths,
    report,
  };
}

async function runDevAllStatus({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStatus({
    profile: "all",
    repoRoot,
  });
}

async function runDevBackendStatus({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStatus({
    profile: "backend",
    repoRoot,
  });
}

async function stopManagedServiceProcesses(managedServices) {
  const stopped = [];
  for (const service of managedServices) {
    // Phase 1: graceful kill via process tree
    if (service.pid && isProcessAlive(service.pid)) {
      killProcessTree(service.pid);
    }

    // Phase 2: wait briefly for port release, then force kill residuals
    if (service.port) {
      await delay(500);
      if (await isPortListening(service.port)) {
        // Port still occupied — find and kill the residual process
        const residualPid = getListeningPidForPort(service.port);
        if (residualPid) {
          forceKillProcessTree(residualPid);
          await delay(300);
        }
      }
    }

    stopped.push(service.name);
  }

  return stopped;
}

function stopManagedInfrastructure(repoRoot, managedServices) {
  const composeServices = managedServices
    .filter((service) => service.composeService)
    .map((service) => service.composeService);

  if (composeServices.length === 0) {
    return { ok: true };
  }

  const composeResult = runCommandSync("docker", ["compose", "stop", ...composeServices], {
    cwd: repoRoot,
    encoding: "utf8",
  });

  if (composeResult.status !== 0) {
    return {
      ok: false,
      detail: composeResult.stderr?.trim() || composeResult.stdout?.trim() || "docker compose stop failed",
    };
  }

  return { ok: true };
}

async function runWorkflowStop({ profile = "all", repoRoot = getRepoRoot() } = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const runtimeState = readRuntimeState(paths.statePath);
  const plan = createStopPlan({ runtimeState });

  const infrastructure = plan.toStop.filter((service) => service.composeService);
  const applications = plan.toStop.filter((service) => !service.composeService);

  if (infrastructure.length > 0 && !canUseDockerCompose()) {
    return {
      ok: false,
      reason: "missing_prerequisite",
      detail: "docker compose is unavailable or Docker Desktop is not ready",
      plan,
    };
  }

  const composeStopResult = stopManagedInfrastructure(repoRoot, infrastructure);
  if (!composeStopResult.ok) {
    return {
      ok: false,
      reason: "docker_compose_failed",
      detail: composeStopResult.detail,
      plan,
    };
  }

  const stopped = await stopManagedServiceProcesses(applications);
  const nextState = {
    ...createEmptyRuntimeState(),
    services: plan.preserved.reduce((acc, service) => {
      acc[service.name] = {
        ...service,
      };
      return acc;
    }, {}),
  };

  writeRuntimeState(paths.statePath, nextState);

  return {
    ok: true,
    stopped: [...infrastructure.map((service) => service.name), ...stopped],
    preserved: plan.preserved.map((service) => service.name),
    paths,
  };
}

async function runDevAllStop({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStop({
    profile: "all",
    repoRoot,
  });
}

async function runDevBackendStop({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowStop({
    profile: "backend",
    repoRoot,
  });
}

function runWorkflowLogs({ profile = "all", repoRoot = getRepoRoot() } = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.all;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const runtimeState = readRuntimeState(paths.statePath);
  const logs = Object.entries(runtimeState.services ?? {}).map(([name, service]) => ({
    name,
    logPath: service.logPath ?? null,
    errorLogPath: service.errorLogPath ?? null,
  }));

  return {
    ok: true,
    paths,
    logs,
  };
}

function runDevAllLogs({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowLogs({
    profile: "all",
    repoRoot,
  });
}

function runDevBackendLogs({ repoRoot = getRepoRoot() } = {}) {
  return runWorkflowLogs({
    profile: "backend",
    repoRoot,
  });
}

function printPrerequisiteVersions(serviceDefinitions) {
  const checks = checkPrerequisiteVersions(serviceDefinitions);
  if (checks.length === 0) return;

  console.log(colorDim("Prerequisites:"));
  for (const check of checks) {
    const icon = check.available ? colorGreen("\u2713") : colorRed("\u2717");
    const ver = check.available ? colorDim(check.version) : colorRed("not found");
    console.log(`  ${icon} ${check.command}: ${ver}`);
  }
  console.log();
}

function printStartResult(result, workflowProfile = WORKFLOW_PROFILES.all) {
  if (!result.ok) {
    console.error(
      `${colorRed("FAIL")} ${workflowProfile.commandLabel} failed for ${colorBold(result.service?.name ?? "workflow")}`,
    );
    console.error(`      ${result.detail}`);
    if (result.logPaths?.stderrPath) {
      const tail = tailFile(result.logPaths.stderrPath, 15);
      if (tail) {
        console.error(colorDim("\n--- last stderr lines ---"));
        console.error(colorDim(tail));
        console.error(colorDim("--- end ---\n"));
      }
      console.error(`      Logs: ${result.logPaths.stdoutPath ?? "-"} / ${result.logPaths.stderrPath ?? "-"}`);
    }
    return 1;
  }

  console.log(`${colorGreen("\u2713")} ${colorBold(workflowProfile.displayName)} workflow ready:\n`);
  for (const service of result.services) {
    const endpoint = service.healthUrl ?? (service.port ? `tcp://127.0.0.1:${service.port}` : "n/a");
    const actionLabel = statusLabel(service.action);
    const hotReload = service.hotReload ? colorCyan(" [hot-reload]") : "";
    console.log(`  ${statusIcon(true)} ${colorBold(service.name)}: ${actionLabel} (${service.source})${hotReload}`);
    console.log(`    ${colorDim(endpoint)}`);
    if (service.logPath || service.errorLogPath) {
      console.log(`    ${colorDim(`logs: ${service.logPath ?? "-"} / ${service.errorLogPath ?? "-"}`)}`);
    }
  }
  console.log(`\n${colorDim(`State: ${result.paths.statePath}`)}`);
  return 0;
}

function printStatusResult(result, workflowProfile = WORKFLOW_PROFILES.all) {
  console.log(`${colorBold(workflowProfile.displayName)} status:\n`);
  for (const service of Object.values(result.report.services)) {
    const endpoint = service.healthUrl ?? (service.port ? `tcp://127.0.0.1:${service.port}` : "n/a");
    const icon = statusIcon(service.status === "ready");
    console.log(
      `  ${icon} ${colorBold(service.name)}: ${statusLabel(service.status)} (${service.source}, ${statusLabel(service.health)})`,
    );
    console.log(`    ${colorDim(endpoint)}`);
    if (service.logPath || service.errorLogPath) {
      console.log(`    ${colorDim(`logs: ${service.logPath ?? "-"} / ${service.errorLogPath ?? "-"}`)}`);
    }
  }
  console.log(`\n${colorDim(`State: ${result.paths.statePath}`)}`);
  return 0;
}

function printStopResult(result, workflowProfile = WORKFLOW_PROFILES.all) {
  if (!result.ok) {
    console.error(`${colorRed("FAIL")} ${workflowProfile.commandLabel}:stop failed: ${result.detail}`);
    return 1;
  }

  if (result.stopped.length > 0) {
    console.log(`${colorGreen("\u2713")} Stopped: ${result.stopped.join(", ")}`);
  } else {
    console.log(`${colorDim("No managed services to stop.")}`);
  }
  if (result.preserved.length > 0) {
    console.log(`${colorYellow("\u21B3")} Preserved (external): ${result.preserved.join(", ")}`);
  }
  console.log(colorDim(`State: ${result.paths.statePath}`));
  return 0;
}

function printLogsResult(result, workflowProfile = WORKFLOW_PROFILES.all) {
  console.log(`${colorBold(workflowProfile.commandLabel)} logs:\n`);
  for (const log of result.logs) {
    console.log(`  ${colorCyan(log.name)}:`);
    console.log(`    stdout: ${log.logPath ?? colorDim("n/a")}`);
    console.log(`    stderr: ${log.errorLogPath ?? colorDim("n/a")}`);
  }
  console.log(`\n${colorDim(`Runtime logs directory: ${result.paths.runtimeLogsDir}`)}`);
  return 0;
}

function printVerifyResult(result, workflowProfile = WORKFLOW_PROFILES.backend) {
  console.log(`${colorBold(workflowProfile.displayName)} verify:\n`);
  for (const stage of result.stages) {
    console.log(`  ${statusIcon(stage.ok)} ${stage.name}: ${stage.detail}`);
    if (stage.endpoint) {
      console.log(`    ${colorDim(`endpoint: ${stage.endpoint}`)}`);
    }
    if (stage.logPath || stage.errorLogPath) {
      console.log(`    ${colorDim(`logs: ${stage.logPath ?? "-"} / ${stage.errorLogPath ?? "-"}`)}`);
    }
    if (stage.fixturePath) {
      console.log(`    ${colorDim(`fixture: ${stage.fixturePath}`)}`);
    }
  }
  console.log(`\n${colorDim(`State: ${result.paths.statePath}`)}`);
  console.log(colorDim("Use pnpm dev:backend:status / logs / stop for follow-up diagnostics."));
  return result.ok ? 0 : 1;
}

async function runWorkflowRestart({
  profile = "backend",
  repoRoot = getRepoRoot(),
  serviceName,
} = {}) {
  const workflowProfile = getWorkflowProfile(profile) ?? WORKFLOW_PROFILES.backend;
  const paths = getWorkflowPathsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });
  const runtimeState = readRuntimeState(paths.statePath);
  const serviceDefinitions = createServiceDefinitionsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
  });

  // Find the target service
  const targetDef = serviceDefinitions.find((s) => s.name === serviceName);
  if (!targetDef) {
    const available = serviceDefinitions.map((s) => s.name).join(", ");
    return {
      ok: false,
      reason: "unknown_service",
      detail: `Unknown service "${serviceName}". Available: ${available}`,
    };
  }

  // Infrastructure services restart via docker-compose
  if (targetDef.kind === "infra") {
    if (targetDef.composeService && canUseDockerCompose()) {
      runCommandSync("docker", ["compose", "restart", targetDef.composeService], {
        cwd: repoRoot,
        encoding: "utf8",
      });
      if (!(await waitForServiceHealth(targetDef, 15000))) {
        return {
          ok: false,
          reason: "restart_unhealthy",
          detail: `${serviceName} did not become healthy after docker-compose restart`,
        };
      }
      return { ok: true, service: serviceName, action: "restarted" };
    }
    return {
      ok: false,
      reason: "infra_no_restart",
      detail: `Cannot restart infra service "${serviceName}" without docker-compose`,
    };
  }

  // Application services: stop then re-launch
  const trackedState = runtimeState.services?.[serviceName];
  if (trackedState?.pid && isProcessAlive(trackedState.pid)) {
    killProcessTree(trackedState.pid);
    await delay(800);
    if (targetDef.port && (await isPortListening(targetDef.port))) {
      const residualPid = getListeningPidForPort(targetDef.port);
      if (residualPid) {
        forceKillProcessTree(residualPid);
        await delay(300);
      }
    }
  }

  // Re-launch
  const result = await ensureApplicationService(targetDef, paths, {
    ...runtimeState,
    services: {
      ...runtimeState.services,
      [serviceName]: undefined,
    },
  });

  if (!result.ok) {
    return {
      ok: false,
      reason: result.reason ?? "restart_failed",
      detail: result.detail ?? `Failed to restart ${serviceName}`,
    };
  }

  // Update state
  const nextState = {
    ...runtimeState,
    services: {
      ...runtimeState.services,
      [serviceName]: result.record,
    },
  };
  writeRuntimeState(paths.statePath, nextState);

  return { ok: true, service: serviceName, action: "restarted" };
}

function printRestartResult(result) {
  if (!result.ok) {
    console.error(`${colorRed("FAIL")} restart failed: ${result.detail}`);
    return 1;
  }
  console.log(`${colorGreen("\u2713")} ${colorBold(result.service)} restarted successfully`);
  return 0;
}

function parseWorkflowCommand(argv = []) {
  const workflowProfile = getWorkflowProfile(argv[0]);
  if (workflowProfile) {
    return {
      workflowProfile,
      command: argv[1] ?? "start",
      extra: argv.slice(2),
    };
  }

  return {
    workflowProfile: WORKFLOW_PROFILES.all,
    command: argv[0] ?? "start",
    extra: argv.slice(1),
  };
}

async function ensureSidecarsPreparedIfRequired({
  workflowProfile,
  repoRoot = getRepoRoot(),
  stdout = (message) => console.log(message),
} = {}) {
  if (!shouldRequirePreparedSidecars()) {
    return { ok: true, required: false, preferPreparedSidecars: false };
  }

  const serviceDefinitions = createServiceDefinitionsForProfile({
    profile: workflowProfile.profile,
    repoRoot,
    preferPreparedSidecars: true,
  });
  const missing = getMissingPreparedSidecars(serviceDefinitions, { repoRoot });

  if (missing.length === 0) {
    stdout(`${colorDim("[dev-all]")} prepared sidecars present for: ${missing.length === 0 ? "all registered services" : missing.join(", ")}`);
    return { ok: true, required: true, preferPreparedSidecars: true };
  }

  stdout(
    `${colorYellow("WARN")} missing prepared sidecar binaries on Windows: ${missing.join(", ")}`,
  );
  stdout(
    `      set AGENTFORGE_DEV_ALLOW_SOURCE_SERVICES=1 to opt out and run via \`go run\` / \`tsx\`.`,
  );

  const prepare = runDesktopDevPrepare({
    repoRoot,
    progress: (message) => stdout(`${colorDim("[dev-all]")} ${message}`),
  });

  if (!prepare.ok) {
    return {
      ok: false,
      reason: "sidecar_prepare_failed",
      detail: prepare.detail,
      required: true,
      preferPreparedSidecars: false,
    };
  }

  return { ok: true, required: true, preferPreparedSidecars: true };
}

async function main(argv = process.argv.slice(2)) {
  const { workflowProfile, command, extra } = parseWorkflowCommand(argv);

  if (command === "watch") {
    process.env.PREFER_AIR = "1";
    const airAvailable = detectAirAvailable();
    if (!airAvailable) {
      console.warn(
        `${colorYellow("WARN")} air is not installed. Install with: go install github.com/air-verse/air@latest`,
      );
      console.warn(`      Falling back to \`go run\` (no hot-reload).`);
    }
    const prep = await ensureSidecarsPreparedIfRequired({ workflowProfile });
    if (!prep.ok) {
      console.error(`${colorRed("FAIL")} ${prep.detail}`);
      return 1;
    }
    return printStartResult(
      await runWorkflowStart({
        profile: workflowProfile.profile,
        preferPreparedSidecars: prep.preferPreparedSidecars,
      }),
      workflowProfile,
    );
  }

  if (command === "start") {
    const prep = await ensureSidecarsPreparedIfRequired({ workflowProfile });
    if (!prep.ok) {
      console.error(`${colorRed("FAIL")} ${prep.detail}`);
      return 1;
    }
    return printStartResult(
      await runWorkflowStart({
        profile: workflowProfile.profile,
        preferPreparedSidecars: prep.preferPreparedSidecars,
      }),
      workflowProfile,
    );
  }

  if (command === "status") {
    return printStatusResult(
      await runWorkflowStatus({
        profile: workflowProfile.profile,
      }),
      workflowProfile,
    );
  }

  if (command === "stop") {
    return printStopResult(
      await runWorkflowStop({
        profile: workflowProfile.profile,
      }),
      workflowProfile,
    );
  }

  if (command === "logs") {
    return printLogsResult(
      runWorkflowLogs({
        profile: workflowProfile.profile,
      }),
      workflowProfile,
    );
  }

  if (command === "verify") {
    return printVerifyResult(
      await runWorkflowVerify({
        profile: workflowProfile.profile,
      }),
      workflowProfile,
    );
  }

  if (command === "restart") {
    const serviceName = extra[0];
    if (!serviceName) {
      console.error(`${colorRed("FAIL")} Usage: ${workflowProfile.commandLabel} restart <service-name>`);
      const defs = createServiceDefinitionsForProfile({ profile: workflowProfile.profile });
      console.error(`       Available services: ${defs.map((s) => s.name).join(", ")}`);
      return 1;
    }
    return printRestartResult(
      await runWorkflowRestart({
        profile: workflowProfile.profile,
        serviceName,
      }),
    );
  }

  console.error(`Unknown ${workflowProfile.commandLabel} command: ${command}`);
  console.error(`Available commands: start, watch, status, stop, restart, logs, verify`);
  return 1;
}

if (require.main === module) {
  void main().then((exitCode) => {
    process.exitCode = exitCode;
  });
}

module.exports = {
  createDevBackendServiceDefinitions,
  createDevAllServiceDefinitions,
  ensureSidecarsPreparedIfRequired,
  getDevBackendPaths,
  getDevAllPaths,
  getMissingPreparedSidecars,
  main,
  runDesktopDevPrepare,
  runDevBackendLogs,
  runDevBackendStart,
  runDevBackendVerify,
  runDevBackendStatus,
  runDevBackendStop,
  runDevAllLogs,
  runDevAllStart,
  runDevAllStatus,
  runDevAllStop,
  runWorkflowRestart,
  shouldRequirePreparedSidecars,
};
