/** @jest-environment node */

/* eslint-disable @typescript-eslint/no-require-imports */

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

describe("IM stub smoke helper", () => {
  test("loads the platform fixture and overrides the command content", () => {
    const { buildStubSmokePayload } = require("./im-stub-smoke.js");

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-imstub-fixture-"));
    const fixturesDir = path.join(repoRoot, "src-im-bridge", "scripts", "smoke", "fixtures");
    fs.mkdirSync(fixturesDir, { recursive: true });
    fs.writeFileSync(
      path.join(fixturesDir, "feishu.json"),
      JSON.stringify({
        content: "/team list",
        user_id: "fixture-user",
        user_name: "Fixture User",
        chat_id: "fixture-chat",
        is_group: true,
      }),
      "utf8",
    );

    const payload = buildStubSmokePayload({
      repoRoot,
      platform: "feishu",
      commandContent: "/agent runtimes",
    });

    expect(payload).toEqual({
      content: "/agent runtimes",
      user_id: "fixture-user",
      user_name: "Fixture User",
      chat_id: "fixture-chat",
      is_group: true,
    });
  });

  test("runs the stub smoke roundtrip through delete, post, and reply capture stages", async () => {
    const { runIMStubSmoke } = require("./im-stub-smoke.js");

    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-imstub-roundtrip-"));
    const fixturesDir = path.join(repoRoot, "src-im-bridge", "scripts", "smoke", "fixtures");
    fs.mkdirSync(fixturesDir, { recursive: true });
    fs.writeFileSync(
      path.join(fixturesDir, "feishu.json"),
      JSON.stringify({
        content: "/team list",
        user_id: "fixture-user",
        user_name: "Fixture User",
        chat_id: "fixture-chat",
        is_group: true,
      }),
      "utf8",
    );

    const fetchImpl = jest
      .fn()
      .mockResolvedValueOnce({ ok: true, json: async () => [] })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ ok: true }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ content: "bridge reply" }] });

    const result = await runIMStubSmoke({
      repoRoot,
      platform: "feishu",
      port: 7780,
      commandContent: "/agent runtimes",
      fetchImpl,
      timeoutMs: 50,
      pollIntervalMs: 1,
    });

    expect(result.ok).toBe(true);
    expect(result.failureStage).toBeNull();
    expect(result.stages).toEqual([
      expect.objectContaining({ name: "stub-command", ok: true }),
      expect.objectContaining({ name: "reply-capture", ok: true }),
    ]);
    expect(fetchImpl).toHaveBeenNthCalledWith(
      2,
      "http://127.0.0.1:7780/test/message",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
