#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const net = require("node:net");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const { getRepoRoot } = require("./plugin-dev-targets.js");

function getWorkflowPaths({
  repoRoot = getRepoRoot(),
  stateFileName = "dev-all-state.json",
} = {}) {
  return {
    repoRoot,
    codexDir: path.join(repoRoot, ".codex"),
    runtimeLogsDir: path.join(repoRoot, ".codex", "runtime-logs"),
    statePath: path.join(repoRoot, ".codex", stateFileName),
  };
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

function canUseDockerCompose() {
  if (!isCommandAvailable("docker")) {
    return false;
  }

  const infoResult = runCommandSync("docker", ["info"], {
    stdio: "ignore",
  });

  return infoResult.status === 0;
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
  writeRuntimeState,
};
