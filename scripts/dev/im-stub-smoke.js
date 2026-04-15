#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");
const { setTimeout: delay } = require("node:timers/promises");
const { getRepoRoot } = require("../plugin/plugin-dev-targets.js");

const DEFAULT_VERIFY_COMMAND_CONTENT = "/agent runtimes";
const DEFAULT_SMOKE_TIMEOUT_MS = 5000;
const DEFAULT_SMOKE_POLL_INTERVAL_MS = 250;

function getDefaultFixturePath({ repoRoot = getRepoRoot(), platform = "feishu" } = {}) {
  return path.join(repoRoot, "src-im-bridge", "scripts", "smoke", "fixtures", `${platform}.json`);
}

function buildStubSmokePayload({
  repoRoot = getRepoRoot(),
  platform = "feishu",
  fixturePath,
  commandContent = DEFAULT_VERIFY_COMMAND_CONTENT,
} = {}) {
  const resolvedFixturePath = fixturePath ?? getDefaultFixturePath({ repoRoot, platform });
  if (!fs.existsSync(resolvedFixturePath)) {
    throw new Error(`Stub smoke fixture not found: ${resolvedFixturePath}`);
  }

  const payload = JSON.parse(fs.readFileSync(resolvedFixturePath, "utf8"));
  return {
    ...payload,
    content: commandContent,
  };
}

async function readJsonResponse(response) {
  if (typeof response.json === "function") {
    return response.json();
  }

  return null;
}

async function sendJsonRequest(fetchImpl, url, init) {
  const response = await fetchImpl(url, init);
  const body = await readJsonResponse(response);
  if (!response.ok) {
    const detail =
      (body && typeof body === "object" && "message" in body && body.message) ||
      (body && typeof body === "object" && "error" in body && body.error) ||
      `${init.method} ${url} failed`;
    throw new Error(String(detail));
  }
  return body;
}

function createStage(name, ok, detail, extras = {}) {
  return {
    name,
    ok,
    detail,
    ...extras,
  };
}

async function runIMStubSmoke({
  repoRoot = getRepoRoot(),
  platform = "feishu",
  port = 7780,
  fixturePath,
  commandContent = DEFAULT_VERIFY_COMMAND_CONTENT,
  fetchImpl = globalThis.fetch,
  timeoutMs = DEFAULT_SMOKE_TIMEOUT_MS,
  pollIntervalMs = DEFAULT_SMOKE_POLL_INTERVAL_MS,
} = {}) {
  if (typeof fetchImpl !== "function") {
    throw new Error("A fetch implementation is required for IM stub smoke");
  }

  const resolvedFixturePath = fixturePath ?? getDefaultFixturePath({ repoRoot, platform });
  const payload = buildStubSmokePayload({
    repoRoot,
    platform,
    fixturePath: resolvedFixturePath,
    commandContent,
  });
  const baseUrl = `http://127.0.0.1:${port}`;
  const stages = [];

  try {
    await sendJsonRequest(fetchImpl, `${baseUrl}/test/replies`, {
      method: "DELETE",
    });
    await sendJsonRequest(fetchImpl, `${baseUrl}/test/message`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });
    stages.push(
      createStage("stub-command", true, `Injected ${commandContent}`, {
        baseUrl,
        fixturePath: resolvedFixturePath,
      }),
    );
  } catch (error) {
    stages.push(
      createStage("stub-command", false, error instanceof Error ? error.message : String(error), {
        baseUrl,
        fixturePath: resolvedFixturePath,
      }),
    );
    return {
      ok: false,
      failureStage: "stub-command",
      baseUrl,
      fixturePath: resolvedFixturePath,
      payload,
      stages,
    };
  }

  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    const replies = await sendJsonRequest(fetchImpl, `${baseUrl}/test/replies`, {
      method: "GET",
    });
    const firstReply = Array.isArray(replies) ? replies[0] : null;
    if (firstReply && typeof firstReply.content === "string" && firstReply.content.trim() !== "") {
      stages.push(
        createStage("reply-capture", true, "Captured non-empty reply", {
          baseUrl,
          fixturePath: resolvedFixturePath,
          firstReply,
          replyCount: replies.length,
        }),
      );
      return {
        ok: true,
        failureStage: null,
        baseUrl,
        fixturePath: resolvedFixturePath,
        payload,
        stages,
        firstReply,
        replies,
      };
    }
    await delay(pollIntervalMs);
  }

  stages.push(
    createStage("reply-capture", false, "No replies captured", {
      baseUrl,
      fixturePath: resolvedFixturePath,
    }),
  );
  return {
    ok: false,
    failureStage: "reply-capture",
    baseUrl,
    fixturePath: resolvedFixturePath,
    payload,
    stages,
  };
}

module.exports = {
  DEFAULT_VERIFY_COMMAND_CONTENT,
  buildStubSmokePayload,
  getDefaultFixturePath,
  runIMStubSmoke,
};
