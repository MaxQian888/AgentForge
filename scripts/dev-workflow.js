#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const net = require("node:net");
const path = require("node:path");
const { spawn, spawnSync } = require("node:child_process");
const { getRepoRoot } = require("./plugin-dev-targets.js");

const DOCKER_INFO_TIMEOUT_MS = 10000;
const DOCKER_DESKTOP_START_TIMEOUT_MS = 10000;
const DOCKER_DESKTOP_STATUS_TIMEOUT_MS = 5000;

function getWorkflowPaths({
  repoRoot = getRepoRoot(),
  stateFileName = "dev-all-state.json",
} = {}) {
  const runtimeBaseDir = resolveWritableRuntimeBaseDir(repoRoot);
  return {
    repoRoot,
    codexDir: runtimeBaseDir,
    runtimeLogsDir: path.join(runtimeBaseDir, "runtime-logs"),
    statePath: path.join(runtimeBaseDir, stateFileName),
  };
}

function resolveWritableRuntimeBaseDir(repoRoot) {
  const candidates = [
    path.join(repoRoot, ".codex"),
    path.join(repoRoot, "tmp-runtime"),
  ];

  for (const candidate of candidates) {
    if (canWriteDirectory(candidate)) {
      return candidate;
    }
  }

  return candidates[candidates.length - 1];
}

function canWriteDirectory(dirPath) {
  try {
    ensureDirectory(dirPath);
    const probePath = path.join(dirPath, `.write-probe-${process.pid}-${Date.now()}.tmp`);
    fs.writeFileSync(probePath, "ok", "utf8");
    fs.unlinkSync(probePath);
    return true;
  } catch {
    return false;
  }
}

function ensureDirectory(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

function createEmptyRuntimeState() {
  return {
    version: 1,
    updatedAt: null,
    services: {},
  };
}

function readRuntimeState(statePath) {
  if (!fs.existsSync(statePath)) {
    return createEmptyRuntimeState();
  }

  try {
    const parsed = JSON.parse(fs.readFileSync(statePath, "utf8"));
    return {
      ...createEmptyRuntimeState(),
      ...parsed,
      services: parsed?.services ?? {},
    };
  } catch {
    return createEmptyRuntimeState();
  }
}

function writeRuntimeState(statePath, runtimeState) {
  ensureDirectory(path.dirname(statePath));
  fs.writeFileSync(
    statePath,
    `${JSON.stringify(
      {
        ...createEmptyRuntimeState(),
        ...runtimeState,
        updatedAt: new Date().toISOString(),
      },
      null,
      2,
    )}\n`,
    "utf8",
  );
}

function getCommandVersionArgs(command) {
  if (command === "go") {
    return ["version"];
  }

  return ["--version"];
}

function getWindowsCommandArgs(command, args) {
  return ["/d", "/s", "/c", [command, ...args].join(" ")];
}

function needsWindowsCmdWrapper(command) {
  return ["pnpm", "npm", "npx", "yarn"].includes(command.toLowerCase());
}

function runCommandSync(command, args, options = {}) {
  if (process.platform === "win32" && needsWindowsCmdWrapper(command)) {
    return spawnSync("cmd.exe", getWindowsCommandArgs(command, args), {
      ...options,
      shell: false,
    });
  }

  return spawnSync(command, args, {
    ...options,
    shell: false,
  });
}

function isCommandAvailable(command, args = getCommandVersionArgs(command)) {
  const result = runCommandSync(command, args, {
    stdio: "ignore",
  });

  return result.status === 0;
}

function getDockerDesktopExecutablePath() {
  if (process.platform !== "win32") {
    return null;
  }

  const candidates = [
    process.env.ProgramFiles
      ? path.join(process.env.ProgramFiles, "Docker", "Docker", "Docker Desktop.exe")
      : null,
    process.env.LocalAppData
      ? path.join(process.env.LocalAppData, "Programs", "Docker", "Docker", "Docker Desktop.exe")
      : null,
  ].filter(Boolean);

  return candidates.find((candidate) => fs.existsSync(candidate)) ?? null;
}

function getDockerComposeAvailability() {
  const dockerDesktopExecutablePath = getDockerDesktopExecutablePath();
  const dockerAvailable = isCommandAvailable("docker");

  if (!dockerAvailable) {
    return {
      ready: false,
      dockerAvailable: false,
      canAutoStart: false,
      reason: "docker_cli_missing",
      detail: "docker CLI is unavailable",
      dockerDesktopExecutablePath,
      infoResult: null,
    };
  }

  const infoResult = runCommandSync("docker", ["info"], {
    encoding: "utf8",
    timeout: DOCKER_INFO_TIMEOUT_MS,
  });

  if (infoResult.status === 0) {
    return {
      ready: true,
      dockerAvailable: true,
      canAutoStart: process.platform === "win32" && Boolean(dockerDesktopExecutablePath),
      reason: null,
      detail: null,
      dockerDesktopExecutablePath,
      infoResult,
    };
  }

  const canAutoStart = process.platform === "win32" && Boolean(dockerDesktopExecutablePath);
  const desktopStatus = canAutoStart ? getDockerDesktopStatus() : null;
  const timeoutDetail =
    infoResult.error?.code === "ETIMEDOUT"
      ? desktopStatus === "starting"
        ? "Docker Desktop is still starting and the docker daemon is not ready yet"
        : "docker daemon did not respond before the readiness timeout"
      : null;
  const daemonDetail =
    infoResult.stderr?.trim() ||
    infoResult.stdout?.trim() ||
    (desktopStatus === "starting"
      ? "Docker Desktop is still starting and the docker daemon is not ready yet"
      : canAutoStart
        ? "Docker Desktop is installed but not ready"
        : "docker daemon is unavailable");

  return {
    ready: false,
    dockerAvailable: true,
    canAutoStart,
    reason: canAutoStart ? "docker_desktop_not_ready" : "docker_daemon_unavailable",
    detail: timeoutDetail ?? daemonDetail,
    desktopStatus,
    dockerDesktopExecutablePath,
    infoResult,
  };
}

function parseDockerDesktopStatus(output = "") {
  const match = output.match(/Status\s+([a-z]+)/iu);
  if (match) {
    return match[1].toLowerCase();
  }

  if (/Could not retrieve status/iu.test(output)) {
    return "stopped";
  }

  return null;
}

function getDockerDesktopStatus() {
  if (process.platform !== "win32" || !isCommandAvailable("docker")) {
    return null;
  }

  const statusResult = runCommandSync("docker", ["desktop", "status"], {
    encoding: "utf8",
    timeout: DOCKER_DESKTOP_STATUS_TIMEOUT_MS,
  });

  return parseDockerDesktopStatus(
    `${statusResult.stdout ?? ""}\n${statusResult.stderr ?? ""}`.trim(),
  );
}

function startDockerDesktop(availability = getDockerComposeAvailability()) {
  if (!availability?.canAutoStart) {
    return {
      ok: false,
      reason: "docker_desktop_unavailable",
      detail: availability?.detail ?? "Docker Desktop auto-start is unavailable",
    };
  }

  const desktopStatus = getDockerDesktopStatus();
  if (desktopStatus && desktopStatus !== "stopped") {
    return {
      ok: true,
      method: "desktop-status-wait",
      desktopStatus,
    };
  }

  const desktopStartResult = runCommandSync("docker", ["desktop", "start"], {
    encoding: "utf8",
    timeout: DOCKER_DESKTOP_START_TIMEOUT_MS,
  });

  if (desktopStartResult.status === 0 || desktopStartResult.error?.code === "ETIMEDOUT") {
    return {
      ok: true,
      method: "docker-desktop-cli",
    };
  }

  if (!availability.dockerDesktopExecutablePath) {
    return {
      ok: false,
      reason: "docker_desktop_start_failed",
      detail:
        desktopStartResult.stderr?.trim() ||
        desktopStartResult.stdout?.trim() ||
        desktopStartResult.error?.message ||
        availability.detail,
    };
  }

  try {
    const child = spawn(availability.dockerDesktopExecutablePath, [], {
      detached: true,
      shell: false,
      stdio: "ignore",
      windowsHide: true,
    });
    child.unref();

    return {
      ok: true,
      method: "desktop-executable",
      pid: child.pid ?? null,
      executablePath: availability.dockerDesktopExecutablePath,
    };
  } catch (error) {
    return {
      ok: false,
      reason: "docker_desktop_start_failed",
      detail:
        desktopStartResult.stderr?.trim() ||
        desktopStartResult.stdout?.trim() ||
        error.message ||
        availability.detail,
    };
  }
}

function canUseDockerCompose() {
  return getDockerComposeAvailability().ready;
}

function isProcessAlive(pid) {
  if (!pid || typeof pid !== "number") {
    return false;
  }

  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function isPortListening(port, host = "127.0.0.1", timeoutMs = 750) {
  return new Promise((resolve) => {
    const socket = net.createConnection({ port, host });
    const finalize = (value) => {
      socket.removeAllListeners();
      socket.destroy();
      resolve(value);
    };

    socket.setTimeout(timeoutMs);
    socket.once("connect", () => finalize(true));
    socket.once("timeout", () => finalize(false));
    socket.once("error", () => finalize(false));
  });
}

async function probeHealthUrl(healthUrl) {
  try {
    const response = await fetch(healthUrl);
    return response.ok;
  } catch {
    return false;
  }
}

async function probeServiceHealth(service) {
  if (service.healthUrl) {
    return probeHealthUrl(service.healthUrl);
  }

  if (service.port) {
    return isPortListening(service.port);
  }

  return false;
}

function normalizeTrackedSource(source, liveHealth) {
  if (source) {
    return source;
  }

  return liveHealth ? "external" : "untracked";
}

function buildServiceStatus(serviceDefinition, trackedState, liveHealth, pidExists) {
  const source = normalizeTrackedSource(trackedState?.source, liveHealth);
  const managed = source === "managed";

  let status = liveHealth ? "ready" : "stopped";
  if (managed && !liveHealth && trackedState?.pid && !pidExists) {
    status = "stale";
  } else if (trackedState?.source && !liveHealth) {
    status = "unhealthy";
  }

  return {
    name: serviceDefinition.name,
    kind: serviceDefinition.kind ?? "application",
    source,
    managed,
    status,
    health: liveHealth ? "healthy" : "unhealthy",
    port: serviceDefinition.port ?? trackedState?.port ?? null,
    healthUrl: serviceDefinition.healthUrl ?? trackedState?.healthUrl ?? null,
    pid: trackedState?.pid ?? null,
    logPath: trackedState?.logPath ?? null,
    errorLogPath: trackedState?.errorLogPath ?? null,
    composeService: trackedState?.composeService ?? serviceDefinition.composeService ?? null,
    startedAt: trackedState?.startedAt ?? null,
    lastKnownStatus: trackedState?.lastKnownStatus ?? null,
  };
}

function reconcileRuntimeState({
  serviceDefinitions,
  runtimeState,
  liveHealthByService = {},
  pidExistsByService = {},
} = {}) {
  const services = {};

  for (const serviceDefinition of serviceDefinitions) {
    const trackedState = runtimeState?.services?.[serviceDefinition.name] ?? null;
    const liveHealth = Boolean(liveHealthByService[serviceDefinition.name]);
    const pidExists =
      typeof pidExistsByService[serviceDefinition.name] === "boolean"
        ? pidExistsByService[serviceDefinition.name]
        : isProcessAlive(trackedState?.pid);

    services[serviceDefinition.name] = buildServiceStatus(
      serviceDefinition,
      trackedState,
      liveHealth,
      pidExists,
    );
  }

  return {
    updatedAt: new Date().toISOString(),
    services,
  };
}

function createStopPlan({ runtimeState } = {}) {
  const toStop = [];
  const preserved = [];

  for (const [name, service] of Object.entries(runtimeState?.services ?? {})) {
    if (service?.source === "managed") {
      toStop.push({
        name,
        ...service,
      });
      continue;
    }

    preserved.push({
      name,
      ...service,
    });
  }

  return { toStop, preserved };
}

module.exports = {
  canUseDockerCompose,
  createEmptyRuntimeState,
  createStopPlan,
  ensureDirectory,
  getDockerComposeAvailability,
  getDockerDesktopExecutablePath,
  getDockerDesktopStatus,
  getCommandVersionArgs,
  getWorkflowPaths,
  isCommandAvailable,
  isPortListening,
  isProcessAlive,
  needsWindowsCmdWrapper,
  probeHealthUrl,
  probeServiceHealth,
  readRuntimeState,
  reconcileRuntimeState,
  runCommandSync,
  startDockerDesktop,
  writeRuntimeState,
};
