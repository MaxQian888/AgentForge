#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const { spawn } = require("node:child_process");
const { join } = require("node:path");
const { setTimeout: delay } = require("node:timers/promises");
const { getRepoRoot } = require("./plugin-dev-targets.js");
const { isCommandAvailable } = require("./dev-workflow.js");

function createServiceDefinitions({ repoRoot = getRepoRoot() } = {}) {
  return [
    {
      name: "go-orchestrator",
      cwd: join(repoRoot, "src-go"),
      command: "go",
      args: ["run", "./cmd/server"],
      env: {
        PORT: "7777",
        BRIDGE_URL: "http://127.0.0.1:7778",
      },
      healthUrl: "http://127.0.0.1:7777/health",
    },
    {
      name: "ts-bridge",
      cwd: join(repoRoot, "src-bridge"),
      command: "bun",
      args: ["run", "dev"],
      env: {
        PORT: "7778",
        GO_API_URL: "http://127.0.0.1:7777",
        GO_WS_URL: "ws://127.0.0.1:7777/ws/bridge",
      },
      healthUrl: "http://127.0.0.1:7778/health",
    },
  ];
}

function collectMissingPrerequisites(checks) {
  return checks.filter((check) => !check.available).map((check) => check.name);
}

async function probeHealth(healthUrl) {
  try {
    const response = await fetch(healthUrl);
    return response.ok;
  } catch {
    return false;
  }
}

async function waitForHealthyService(healthUrl, timeoutMs = 30000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await probeHealth(healthUrl)) {
      return true;
    }
    await delay(1000);
  }
  return false;
}

async function ensureService(service) {
  if (await probeHealth(service.healthUrl)) {
    return { name: service.name, status: "reused" };
  }

  let child;
  try {
    child = spawn(service.command, service.args, {
      cwd: service.cwd,
      detached: true,
      shell: process.platform === "win32",
      stdio: "ignore",
      env: {
        ...process.env,
        ...service.env,
      },
    });
  } catch (error) {
    return {
      name: service.name,
      status: "missing_prerequisite",
      detail: error instanceof Error ? error.message : String(error),
    };
  }

  child.unref();

  if (await waitForHealthyService(service.healthUrl)) {
    return { name: service.name, status: "started" };
  }

  return {
    name: service.name,
    status: "unhealthy",
    detail: `Service did not become healthy at ${service.healthUrl}`,
  };
}

async function runPluginDevStack({ repoRoot = getRepoRoot() } = {}) {
  const services = createServiceDefinitions({ repoRoot });
  const missing = collectMissingPrerequisites(
    [...new Set(services.map((service) => service.command))].map((command) => ({
      name: command,
      available: isCommandAvailable(command),
    })),
  );

  if (missing.length > 0) {
    return {
      ok: false,
      status: "missing_prerequisites",
      missing,
      services: [],
    };
  }

  const results = [];
  for (const service of services) {
    const result = await ensureService(service);
    results.push(result);
    if (result.status === "missing_prerequisite" || result.status === "unhealthy") {
      return {
        ok: false,
        status: result.status,
        missing: [],
        services: results,
      };
    }
  }

  return {
    ok: true,
    status: "ready",
    missing: [],
    services: results,
    endpoints: services.map((service) => ({
      name: service.name,
      healthUrl: service.healthUrl,
    })),
  };
}

async function main() {
  const result = await runPluginDevStack();

  if (!result.ok) {
    if (result.status === "missing_prerequisites") {
      console.error(`Missing prerequisites: ${result.missing.join(", ")}`);
    } else {
      const failedService = result.services[result.services.length - 1];
      console.error(
        `${failedService?.name ?? "service"} failed: ${failedService?.detail ?? result.status}`,
      );
    }
    process.exit(1);
  }

  console.log("Plugin development stack ready:");
  for (const endpoint of result.endpoints) {
    console.log(`- ${endpoint.name}: ${endpoint.healthUrl}`);
  }
}

if (require.main === module) {
  void main();
}

module.exports = {
  collectMissingPrerequisites,
  createServiceDefinitions,
  ensureService,
  main,
  probeHealth,
  runPluginDevStack,
  waitForHealthyService,
};
