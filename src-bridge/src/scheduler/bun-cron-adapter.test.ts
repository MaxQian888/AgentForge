import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mkdir, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { BunSchedulerAdapter } from "./bun-cron-adapter.js";

const stateDir = join(tmpdir(), "agentforge-bridge-scheduler-tests");

beforeEach(async () => {
  await rm(stateDir, { recursive: true, force: true });
  await mkdir(stateDir, { recursive: true });
});

afterEach(async () => {
  await rm(stateDir, { recursive: true, force: true });
});

describe("BunSchedulerAdapter", () => {
  test("registers enabled os_registered jobs through Bun.cron", async () => {
    const registered: Array<{ path: string; schedule: string; title: string }> = [];
    const adapter = new BunSchedulerAdapter({
      mode: "desktop",
      stateDir,
      fetchImpl: async () =>
        ({
          ok: true,
          status: 200,
          async json() {
            return [
              {
                jobKey: "task-progress-detector",
                schedule: "*/5 * * * *",
                enabled: true,
                executionMode: "os_registered",
              },
              {
                jobKey: "cost-reconcile",
                schedule: "*/15 * * * *",
                enabled: true,
                executionMode: "in_process",
              },
            ];
          },
          async text() {
            return "";
          },
        }) as Response,
      cronRegistrar: async (path, schedule, title) => {
        registered.push({ path, schedule, title });
      },
      cronRemover: async () => undefined,
    });

    await adapter.reconcile();

    expect(registered).toHaveLength(1);
    expect(registered[0]).toEqual(
      expect.objectContaining({
        schedule: "*/5 * * * *",
        title: "agentforge-scheduler-task-progress-detector",
      }),
    );
    expect(adapter.getSnapshot()).toHaveLength(1);
    const worker = await Bun.file(registered[0]!.path).text();
    expect(worker).toContain("/internal/scheduler/jobs/${JOB_KEY}/trigger");
  });

  test("updates changed schedules and removes disabled jobs on reconcile", async () => {
    const responses = [
      [
        {
          jobKey: "task-progress-detector",
          schedule: "*/5 * * * *",
          enabled: true,
          executionMode: "os_registered",
        },
        {
          jobKey: "worktree-garbage-collector",
          schedule: "0 * * * *",
          enabled: true,
          executionMode: "os_registered",
        },
      ],
      [
        {
          jobKey: "task-progress-detector",
          schedule: "0 * * * *",
          enabled: true,
          executionMode: "os_registered",
        },
      ],
    ];
    const registered: Array<{ path: string; schedule: string; title: string }> = [];
    const removed: string[] = [];
    const adapter = new BunSchedulerAdapter({
      mode: "desktop",
      stateDir,
      fetchImpl: async () =>
        ({
          ok: true,
          status: 200,
          async json() {
            return responses.shift() ?? [];
          },
          async text() {
            return "";
          },
        }) as Response,
      cronRegistrar: async (path, schedule, title) => {
        registered.push({ path, schedule, title });
      },
      cronRemover: async (title) => {
        removed.push(title);
      },
    });

    await adapter.reconcile();
    await adapter.reconcile();

    expect(registered).toHaveLength(3);
    expect(removed).toContain("agentforge-scheduler-worktree-garbage-collector");
    expect(removed).toContain("agentforge-scheduler-task-progress-detector");
    expect(adapter.getSnapshot()).toEqual([
      expect.objectContaining({
        jobKey: "task-progress-detector",
        schedule: "0 * * * *",
      }),
    ]);
  });

  test("cleans up existing OS registrations when mode changes away from desktop", async () => {
    const removed: string[] = [];
    const adapter = new BunSchedulerAdapter({
      mode: "desktop",
      stateDir,
      fetchImpl: async () =>
        ({
          ok: true,
          status: 200,
          async json() {
            return [
              {
                jobKey: "task-progress-detector",
                schedule: "*/5 * * * *",
                enabled: true,
                executionMode: "os_registered",
              },
            ];
          },
          async text() {
            return "";
          },
        }) as Response,
      cronRegistrar: async () => undefined,
      cronRemover: async (title) => {
        removed.push(title);
      },
    });

    await adapter.reconcile();
    await adapter.setMode("server");

    expect(removed).toEqual(["agentforge-scheduler-task-progress-detector"]);
    expect(adapter.getSnapshot()).toHaveLength(0);
  });
});
