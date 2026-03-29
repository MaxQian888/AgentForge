#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");
const { spawn } = require("node:child_process");
const { setTimeout: delay } = require("node:timers/promises");
const { getRepoRoot } = require("./plugin-dev-targets.js");
const {
  canUseDockerCompose,
  createEmptyRuntimeState,
  createStopPlan,
  ensureDirectory,
  getDockerComposeAvailability,
  getWorkflowPaths,
  isCommandAvailable,
  isPortListening,
  isProcessAlive,
  probeServiceHealth,
  readRuntimeState,
  reconcileRuntimeState,
  runCommandSync,
  startDockerDesktop,
  writeRuntimeState,
} = require("./dev-workflow.js");

function getDevAllPaths({ repoRoot = getRepoRoot() } = {}) {
  return getWorkflowPaths({
    repoRoot,
    stateFileName: "dev-all-state.json",
  });
}

function createDevAllServiceDefinitions({ repoRoot = getRepoRoot() } = {}) {
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
      start: {
        source: "spawn",
        command: "go",
        args: ["run", "./cmd/server"],
        env: {
          ENV: "development",
          PORT: "7777",
          GOCACHE: path.join(repoRoot, "src-go", ".gocache"),
          GOFLAGS: "-p=1",
          POSTGRES_URL: "postgres://dev:dev@127.0.0.1:5432/appdb?sslmode=disable",
          REDIS_URL: "redis://127.0.0.1:6379",
          BRIDGE_URL: "http://127.0.0.1:7778",
        },
      },
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
  ];
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

async function ensureInfrastructure(repoRoot, services, runtimeState) {
  const results = [];
  const missingInfra = [];

  for (const service of services) {
    const trackedState = runtimeState.services?.[service.name];
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

    if (await isPortListening(service.port)) {
      return {
        ok: false,
        reason: "external_unknown_listener",
        detail: `Port ${service.port} is occupied but ${service.name} is not responding as expected`,
        service,
        results,
      };
    }

    missingInfra.push(service);
  }

  if (missingInfra.length === 0) {
    return { ok: true, results };
  }

  const dockerComposeReady = await ensureDockerComposeReady();
  if (!dockerComposeReady.ok) {
    return {
      ok: false,
      reason: dockerComposeReady.reason ?? "missing_prerequisite",
      detail:
        dockerComposeReady.detail ?? "docker compose is unavailable or Docker Desktop is not ready",
      service: missingInfra[0],
      results,
    };
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

async function runDevAllStart({ repoRoot = getRepoRoot() } = {}) {
  const paths = getDevAllPaths({ repoRoot });
  ensureDirectory(paths.codexDir);
  ensureDirectory(paths.runtimeLogsDir);

  const runtimeState = readRuntimeState(paths.statePath);
  const serviceDefinitions = createDevAllServiceDefinitions({ repoRoot });
  const infrastructureServices = getInfrastructureServices(serviceDefinitions);
  const applicationServices = getApplicationServices(serviceDefinitions);

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
    })),
  };
}

async function collectLiveHealth(serviceDefinitions) {
  const liveHealthByService = {};
  const pidExistsByService = {};

  for (const service of serviceDefinitions) {
    liveHealthByService[service.name] = await probeServiceHealth(service);
  }

  return { liveHealthByService, pidExistsByService };
}

async function runDevAllStatus({ repoRoot = getRepoRoot() } = {}) {
  const paths = getDevAllPaths({ repoRoot });
  const runtimeState = readRuntimeState(paths.statePath);
  const serviceDefinitions = createDevAllServiceDefinitions({ repoRoot });
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

function getListeningPidForPort(port) {
  if (process.platform !== "win32") {
    return null;
  }

  const result = runCommandSync(
    "cmd.exe",
    ["/d", "/s", "/c", `netstat -ano | findstr LISTENING | findstr :${port}`],
    {
      encoding: "utf8",
    },
  );

  if (result.status !== 0) {
    return null;
  }

  const lines = (result.stdout ?? "")
    .split(/\r?\n/u)
    .map((line) => line.trim())
    .filter(Boolean);
  const firstLine = lines[0] ?? "";
  const parts = firstLine.split(/\s+/u);
  const parsed = Number.parseInt(parts[parts.length - 1] ?? "", 10);
  return Number.isFinite(parsed) ? parsed : null;
}

async function stopManagedServiceProcesses(managedServices) {
  const stopped = [];
  for (const service of managedServices) {
    if (service.pid && isProcessAlive(service.pid)) {
      if (process.platform === "win32") {
        try {
          process.kill(service.pid);
        } catch {
          // ignore and let the listener-pid fallback handle any surviving process
        }
      } else {
        try {
          process.kill(service.pid);
        } catch {
          // ignore and let status reconciliation pick up the stale process later
        }
      }
    }

    if (process.platform === "win32" && service.port) {
      const portOwnerPid = getListeningPidForPort(service.port);
      if (portOwnerPid && portOwnerPid !== service.pid) {
        try {
          process.kill(portOwnerPid);
        } catch {
          // ignore and let status reconciliation report any remaining listener
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

async function runDevAllStop({ repoRoot = getRepoRoot() } = {}) {
  const paths = getDevAllPaths({ repoRoot });
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

function runDevAllLogs({ repoRoot = getRepoRoot() } = {}) {
  const paths = getDevAllPaths({ repoRoot });
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

function printStartResult(result) {
  if (!result.ok) {
    console.error(`dev:all failed for ${result.service?.name ?? "workflow"}: ${result.detail}`);
    if (result.logPaths?.stdoutPath || result.logPaths?.stderrPath) {
      console.error(`Logs: ${result.logPaths?.stdoutPath ?? "-"} / ${result.logPaths?.stderrPath ?? "-"}`);
    }
    return 1;
  }

  console.log("Full-stack local development workflow ready:");
  for (const service of result.services) {
    const endpoint = service.healthUrl ?? (service.port ? `tcp://127.0.0.1:${service.port}` : "n/a");
    console.log(`- ${service.name}: ${service.action} (${service.source}) -> ${endpoint}`);
    if (service.logPath || service.errorLogPath) {
      console.log(`  logs: ${service.logPath ?? "-"} / ${service.errorLogPath ?? "-"}`);
    }
  }
  console.log(`State: ${result.paths.statePath}`);
  return 0;
}

function printStatusResult(result) {
  console.log("Full-stack local development status:");
  for (const service of Object.values(result.report.services)) {
    const endpoint = service.healthUrl ?? (service.port ? `tcp://127.0.0.1:${service.port}` : "n/a");
    console.log(
      `- ${service.name}: ${service.status} (${service.source}, ${service.health}) -> ${endpoint}`,
    );
    if (service.logPath || service.errorLogPath) {
      console.log(`  logs: ${service.logPath ?? "-"} / ${service.errorLogPath ?? "-"}`);
    }
  }
  console.log(`State: ${result.paths.statePath}`);
  return 0;
}

function printStopResult(result) {
  if (!result.ok) {
    console.error(`dev:all:stop failed: ${result.detail}`);
    return 1;
  }

  console.log(`Stopped managed services: ${result.stopped.join(", ") || "none"}`);
  console.log(`Preserved reused/external services: ${result.preserved.join(", ") || "none"}`);
  console.log(`State: ${result.paths.statePath}`);
  return 0;
}

function printLogsResult(result) {
  console.log("Known dev:all logs:");
  for (const log of result.logs) {
    console.log(`- ${log.name}: ${log.logPath ?? "-"} / ${log.errorLogPath ?? "-"}`);
  }
  console.log(`Runtime logs directory: ${result.paths.runtimeLogsDir}`);
  return 0;
}

async function main(argv = process.argv.slice(2)) {
  const command = argv[0] ?? "start";

  if (command === "start") {
    return printStartResult(await runDevAllStart());
  }

  if (command === "status") {
    return printStatusResult(await runDevAllStatus());
  }

  if (command === "stop") {
    return printStopResult(await runDevAllStop());
  }

  if (command === "logs") {
    return printLogsResult(runDevAllLogs());
  }

  console.error(`Unknown dev-all command: ${command}`);
  return 1;
}

if (require.main === module) {
  void main().then((exitCode) => {
    process.exitCode = exitCode;
  });
}

module.exports = {
  createDevAllServiceDefinitions,
  getDevAllPaths,
  main,
  runDevAllLogs,
  runDevAllStart,
  runDevAllStatus,
  runDevAllStop,
};
