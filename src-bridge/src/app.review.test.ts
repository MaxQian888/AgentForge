import { describe, expect, test } from "bun:test";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { createApp } from "./server.js";

describe("POST /bridge/review", () => {
  test("returns an aggregated deep review result", async () => {
    const app = createApp({
      pool: new RuntimePoolManager(4),
      streamer: {
        connect() {},
        send() {},
        close() {},
      },
    });

    const response = await app.request("/bridge/review", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        review_id: "review-1",
        task_id: "task-1",
        pr_url: "https://github.com/acme/project/pull/12",
        pr_number: 12,
        title: "Add auth flow",
        description: "Implements auth and adds eval()",
        diff: "const token = process.env.API_TOKEN; eval(userInput);",
        trigger_event: "pull_request.updated",
        changed_files: ["src/auth.ts"],
        dimensions: ["logic", "security", "performance", "compliance"],
      }),
    });

    expect(response.status).toBe(200);
    const payload = await response.json();
    expect(payload.recommendation).toBe("request_changes");
    expect(payload.findings.length).toBeGreaterThan(0);
    expect(payload.dimension_results).toHaveLength(4);
  });
});
