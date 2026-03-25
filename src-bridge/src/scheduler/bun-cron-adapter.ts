import { createHash } from "node:crypto";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

export type BridgeSchedulerMode = "disabled" | "server" | "container" | "local" | "desktop";

export interface SchedulerJobRecord {
  jobKey: string;
  schedule: string;
  enabled: boolean;
  executionMode: string;
}

interface RegisteredCronJob {
  jobKey: string;
  title: string;
  schedule: string;
  scriptPath: string;
  workerHash: string;
}

interface FetchResponseLike {
  ok: boolean;
  status: number;
  json(): Promise<unknown>;
  text(): Promise<string>;
}

type FetchLike = (input: string, init?: RequestInit) => Promise<FetchResponseLike>;
type CronRegistrar = (path: string, schedule: string, title: string) => Promise<void>;
type CronRemover = (title: string) => Promise<void>;

export interface BunSchedulerAdapterOptions {
  goApiUrl?: string;
  mode?: BridgeSchedulerMode;
  pollIntervalMs?: number;
  stateDir?: string;
  titlePrefix?: string;
  internalToken?: string;
  fetchImpl?: FetchLike;
  cronRegistrar?: CronRegistrar;
  cronRemover?: CronRemover;
  logger?: Pick<Console, "warn" | "error" | "info">;
}

export class BunSchedulerAdapter {
  private readonly goApiUrl: string;
  private mode: BridgeSchedulerMode;
  private readonly pollIntervalMs: number;
  private readonly stateDir: string;
  private readonly titlePrefix: string;
  private readonly internalToken: string;
  private readonly fetchImpl: FetchLike;
  private readonly cronRegistrar: CronRegistrar;
  private readonly cronRemover: CronRemover;
  private readonly logger: Pick<Console, "warn" | "error" | "info">;
  private readonly registeredJobs = new Map<string, RegisteredCronJob>();
  private reconcileTimer: ReturnType<typeof setInterval> | null = null;
  private reconcilePromise: Promise<void> | null = null;
  private started = false;

  constructor(options: BunSchedulerAdapterOptions = {}) {
    this.goApiUrl = normalizeBaseUrl(options.goApiUrl ?? process.env.GO_API_URL ?? "http://localhost:7777");
    this.mode = options.mode ?? resolveSchedulerMode();
    this.pollIntervalMs = Math.max(0, options.pollIntervalMs ?? parseInt(process.env.BRIDGE_SCHEDULER_RECONCILE_MS ?? "30000", 10));
    this.stateDir = options.stateDir ?? join(tmpdir(), "agentforge-bridge-scheduler");
    this.titlePrefix = options.titlePrefix ?? "agentforge-scheduler-";
    this.internalToken = options.internalToken ?? process.env.AGENTFORGE_TOKEN ?? "";
    this.fetchImpl = options.fetchImpl ?? (fetch as unknown as FetchLike);
    this.cronRegistrar = options.cronRegistrar ?? ((path, schedule, title) => Bun.cron(path, schedule, title));
    this.cronRemover = options.cronRemover ?? ((title) => Bun.cron.remove(title));
    this.logger = options.logger ?? console;
  }

  async start(): Promise<void> {
    if (this.started) {
      return;
    }
    this.started = true;

    if (this.pollIntervalMs > 0) {
      this.reconcileTimer = setInterval(() => {
        void this.reconcile().catch((error) => {
          this.logger.warn(`[Bridge] scheduler reconcile failed: ${error instanceof Error ? error.message : String(error)}`);
        });
      }, this.pollIntervalMs);
      this.reconcileTimer.unref?.();
    }

    await this.reconcile();
  }

  async stop(): Promise<void> {
    if (this.reconcileTimer) {
      clearInterval(this.reconcileTimer);
      this.reconcileTimer = null;
    }
    this.started = false;
  }

  async setMode(mode: BridgeSchedulerMode): Promise<void> {
    this.mode = mode;
    await this.reconcile();
  }

  async reconcile(): Promise<void> {
    if (this.reconcilePromise) {
      return this.reconcilePromise;
    }

    this.reconcilePromise = this.performReconcile().finally(() => {
      this.reconcilePromise = null;
    });
    return this.reconcilePromise;
  }

  getSnapshot(): RegisteredCronJob[] {
    return [...this.registeredJobs.values()].sort((left, right) => left.jobKey.localeCompare(right.jobKey));
  }

  private async performReconcile(): Promise<void> {
    if (!supportsOSScheduling(this.mode)) {
      await this.cleanupAll();
      return;
    }

    const jobs = await this.fetchJobs();
    const desiredJobs = jobs.filter((job) => job.enabled && job.executionMode === "os_registered");
    const desiredKeys = new Set(desiredJobs.map((job) => job.jobKey));

    for (const [jobKey, existing] of this.registeredJobs) {
      if (desiredKeys.has(jobKey)) {
        continue;
      }
      await this.removeRegistration(existing);
    }

    if (desiredJobs.length === 0) {
      return;
    }

    await mkdir(this.stateDir, { recursive: true });
    for (const job of desiredJobs) {
      await this.upsertRegistration(job);
    }
  }

  private async fetchJobs(): Promise<SchedulerJobRecord[]> {
    const headers = new Headers();
    if (this.internalToken) {
      headers.set("Authorization", `Bearer ${this.internalToken}`);
    }

    const response = await this.fetchImpl(`${this.goApiUrl}/internal/scheduler/jobs`, {
      method: "GET",
      headers,
    });
    if (!response.ok) {
      throw new Error(`scheduler list request failed with status ${response.status}: ${await response.text()}`);
    }

    const payload = await response.json();
    if (!Array.isArray(payload)) {
      throw new Error("scheduler list response must be an array");
    }

    return payload
      .filter((entry): entry is Record<string, unknown> => entry !== null && typeof entry === "object")
      .map((entry) => ({
        jobKey: String(entry.jobKey ?? ""),
        schedule: String(entry.schedule ?? ""),
        enabled: Boolean(entry.enabled),
        executionMode: String(entry.executionMode ?? ""),
      }))
      .filter((entry) => entry.jobKey.length > 0 && entry.schedule.length > 0);
  }

  private async upsertRegistration(job: SchedulerJobRecord): Promise<void> {
    const title = `${this.titlePrefix}${job.jobKey}`;
    const scriptPath = join(this.stateDir, `${job.jobKey}.cron.mjs`);
    const workerSource = renderWorkerScript({
      goApiUrl: this.goApiUrl,
      jobKey: job.jobKey,
      internalToken: this.internalToken,
    });
    const workerHash = hashWorkerSource(workerSource);
    const existing = this.registeredJobs.get(job.jobKey);
    if (
      existing &&
      existing.schedule === job.schedule &&
      existing.workerHash === workerHash
    ) {
      return;
    }

    await writeFile(scriptPath, workerSource, "utf8");
    if (existing) {
      await this.removeRegistration(existing);
    }

    await this.cronRegistrar(scriptPath, job.schedule, title);
    this.registeredJobs.set(job.jobKey, {
      jobKey: job.jobKey,
      title,
      schedule: job.schedule,
      scriptPath,
      workerHash,
    });
  }

  private async removeRegistration(record: RegisteredCronJob): Promise<void> {
    await this.cronRemover(record.title);
    await rm(record.scriptPath, { force: true });
    this.registeredJobs.delete(record.jobKey);
  }

  private async cleanupAll(): Promise<void> {
    for (const record of [...this.registeredJobs.values()]) {
      await this.removeRegistration(record);
    }
  }
}

function hashWorkerSource(value: string): string {
  return createHash("sha1").update(value).digest("hex");
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, "");
}

function supportsOSScheduling(mode: BridgeSchedulerMode): boolean {
  return mode === "desktop" || mode === "local";
}

function resolveSchedulerMode(): BridgeSchedulerMode {
  const rawMode = (process.env.BRIDGE_SCHEDULER_MODE ?? "disabled").trim().toLowerCase();
  switch (rawMode) {
    case "desktop":
    case "local":
    case "server":
    case "container":
      return rawMode;
    default:
      return "disabled";
  }
}

function renderWorkerScript(input: {
  goApiUrl: string;
  jobKey: string;
  internalToken: string;
}): string {
  const baseUrl = JSON.stringify(normalizeBaseUrl(input.goApiUrl));
  const jobKey = JSON.stringify(input.jobKey);
  const token = JSON.stringify(input.internalToken);

  return `const GO_API_URL = ${baseUrl};
const JOB_KEY = ${jobKey};
const INTERNAL_TOKEN = ${token};

async function invokeSchedulerJob() {
  const headers = { "content-type": "application/json" };
  if (INTERNAL_TOKEN) {
    headers.Authorization = \`Bearer \${INTERNAL_TOKEN}\`;
  }

  const response = await fetch(\`\${GO_API_URL}/internal/scheduler/jobs/\${JOB_KEY}/trigger\`, {
    method: "POST",
    headers,
    body: "{}",
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(\`scheduler trigger failed for \${JOB_KEY}: \${response.status} \${body}\`);
  }
}

export default {
  async scheduled() {
    await invokeSchedulerJob();
  },
};
`;
}
