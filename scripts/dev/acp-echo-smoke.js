#!/usr/bin/env node
/* eslint-disable @typescript-eslint/no-require-imports */

/**
 * ACP echo smoke — 5-adapter manual verification step for `pnpm dev:backend:verify`.
 *
 * Gated by VERIFY_ACP=1 (default OFF). When set, sends "echo hello" to each of
 * the five ACP adapters via the bridge's /bridge/execute endpoint and asserts at
 * least one output event is received.
 *
 * This step requires:
 *   - The ts-bridge to be running (http://localhost:7778 by default).
 *   - Real agent CLIs / API keys installed and configured for the adapters under test.
 *
 * Failures are logged loudly but do NOT abort verification of remaining adapters.
 *
 * Adapter-specific key/auth requirements:
 *   - claude_code  → ANTHROPIC_API_KEY
 *   - codex        → OPENAI_API_KEY
 *   - opencode     → opencode CLI installed and configured
 *   - cursor       → cursor-agent CLI installed, user logged in
 *   - gemini       → gemini CLI installed and configured
 */

const { setTimeout: delay } = require("node:timers/promises");
const crypto = require("node:crypto");
const os = require("node:os");
const fs = require("node:fs");
const path = require("node:path");

const VERIFY_ACP = process.env.VERIFY_ACP === "1";
const BRIDGE_PORT = Number(process.env.BRIDGE_PORT ?? 7778);
const BRIDGE_BASE_URL = process.env.BRIDGE_BASE_URL ?? `http://127.0.0.1:${BRIDGE_PORT}`;
const ADAPTER_TIMEOUT_MS = Number(process.env.ACP_ECHO_TIMEOUT_MS ?? 60_000);
const POLL_INTERVAL_MS = 500;

const ADAPTERS = ["claude_code", "codex", "opencode", "cursor", "gemini"];

const isTTY = process.stdout.isTTY && !process.env.NO_COLOR;

function colorGreen(t) { return isTTY ? `\x1b[32m${t}\x1b[0m` : t; }
function colorRed(t) { return isTTY ? `\x1b[31m${t}\x1b[0m` : t; }
function colorDim(t) { return isTTY ? `\x1b[2m${t}\x1b[0m` : t; }

function createStage(name, ok, detail, extras = {}) {
  return { name, ok, detail, ...extras };
}

/**
 * POST /bridge/execute and poll /bridge/status/:task_id until an output event
 * appears or the adapter reports terminal status (end_turn / error / cancelled).
 *
 * Returns { ok, stopReason, outputEventCount, error }.
 */
async function smokeAdapter({ adapterId, fetchImpl, tmpDir }) {
  const taskId = `acp-echo-${adapterId}-${crypto.randomUUID()}`;

  const executeUrl = `${BRIDGE_BASE_URL}/bridge/execute`;
  const statusUrl = `${BRIDGE_BASE_URL}/bridge/status/${taskId}`;

  // POST execute
  let executeRes;
  try {
    executeRes = await fetchImpl(executeUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        task_id: taskId,
        runtime: adapterId,
        prompt: "echo hello",
        worktree_path: tmpDir,
        system_prompt: "You are a coding agent.",
      }),
    });
  } catch (err) {
    return {
      ok: false,
      stopReason: null,
      outputEventCount: 0,
      error: `POST /bridge/execute failed: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  if (!executeRes.ok) {
    let body = "";
    try { body = await executeRes.text(); } catch { /* ignore */ }
    return {
      ok: false,
      stopReason: null,
      outputEventCount: 0,
      error: `POST /bridge/execute returned ${executeRes.status}: ${body.slice(0, 200)}`,
    };
  }

  // Poll /bridge/status/:task_id
  const startedAt = Date.now();
  let outputEventCount = 0;
  let stopReason = null;

  while (Date.now() - startedAt < ADAPTER_TIMEOUT_MS) {
    await delay(POLL_INTERVAL_MS);

    let statusRes;
    try {
      statusRes = await fetchImpl(statusUrl);
    } catch {
      // Transient network error — keep polling.
      continue;
    }

    if (!statusRes.ok) continue;

    let status;
    try {
      status = await statusRes.json();
    } catch {
      continue;
    }

    // Count output events accumulated so far.
    if (Array.isArray(status.events)) {
      outputEventCount = status.events.filter(
        (ev) => ev && (ev.type === "output" || ev.type === "partial_message"),
      ).length;
    }

    // Check for terminal state.
    const terminalStates = new Set(["end_turn", "max_turns", "cancelled", "error", "done", "completed"]);
    const runtimeStatus = status.runtime_status ?? status.status;
    if (runtimeStatus && terminalStates.has(runtimeStatus)) {
      stopReason = runtimeStatus;
      break;
    }

    // Some bridges return stop_reason directly on the status object.
    if (status.stop_reason) {
      stopReason = status.stop_reason;
      break;
    }
  }

  if (!stopReason) {
    // Timed out — attempt to cancel the task so it doesn't linger.
    try {
      await fetchImpl(`${BRIDGE_BASE_URL}/bridge/cancel`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ task_id: taskId }),
      });
    } catch { /* best-effort */ }

    return {
      ok: false,
      stopReason: null,
      outputEventCount,
      error: `Timed out after ${ADAPTER_TIMEOUT_MS}ms waiting for terminal status`,
    };
  }

  const ok = outputEventCount > 0 && stopReason === "end_turn";
  return {
    ok,
    stopReason,
    outputEventCount,
    error: ok ? null : `stopReason=${stopReason}, outputEventCount=${outputEventCount}`,
  };
}

/**
 * Run the ACP echo smoke step for all 5 adapters.
 *
 * @returns {{ ok: boolean; stages: Array; skipped: boolean }}
 */
async function runAcpEchoSmoke({ fetchImpl = globalThis.fetch } = {}) {
  if (!VERIFY_ACP) {
    return {
      ok: true,
      skipped: true,
      stages: [
        createStage(
          "acp-echo-smoke",
          true,
          "ACP echo smoke skipped (VERIFY_ACP not set to 1)",
          { skipped: true },
        ),
      ],
    };
  }

  if (typeof fetchImpl !== "function") {
    return {
      ok: false,
      skipped: false,
      stages: [
        createStage("acp-echo-smoke", false, "fetch is not available — Node 18+ required"),
      ],
    };
  }

  const stages = [];
  let anyFailed = false;

  // Create a shared tmp dir for all adapters.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "acp-echo-verify-"));

  for (const adapterId of ADAPTERS) {
    process.stdout.write(`  ${colorDim(`acp-echo/${adapterId}`)} … `);
    const result = await smokeAdapter({ adapterId, fetchImpl, tmpDir });

    if (result.ok) {
      process.stdout.write(`${colorGreen("ok")} (${result.stopReason}, events=${result.outputEventCount})\n`);
      stages.push(
        createStage(
          `acp-echo/${adapterId}`,
          true,
          `echo hello → ${result.stopReason} (${result.outputEventCount} output events)`,
        ),
      );
    } else {
      process.stdout.write(`${colorRed("FAIL")}\n`);
      console.error(
        `  ${colorRed("!")} acp-echo/${adapterId}: ${result.error ?? "unknown failure"}`,
      );
      stages.push(
        createStage(
          `acp-echo/${adapterId}`,
          false,
          result.error ?? "unknown failure",
        ),
      );
      anyFailed = true;
      // Continue to next adapter — do not abort.
    }
  }

  // Clean up tmp dir (best-effort).
  try { fs.rmSync(tmpDir, { recursive: true, force: true }); } catch { /* ignore */ }

  return {
    ok: !anyFailed,
    skipped: false,
    stages,
    failedAdapters: stages.filter((s) => !s.ok).map((s) => s.name),
  };
}

module.exports = {
  runAcpEchoSmoke,
  VERIFY_ACP,
};
