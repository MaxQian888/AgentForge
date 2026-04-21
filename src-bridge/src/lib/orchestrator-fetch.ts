/**
 * orchestrator-fetch.ts
 *
 * Helpers for outbound requests from the ts-bridge to the Go orchestrator.
 * Automatically attaches X-Trace-ID from the Hono context variable so that
 * traces can be correlated end-to-end across service boundaries.
 */

/**
 * Shape of the minimal context this helper needs. Accepting a narrower type
 * makes unit-testing easier and keeps the helper reusable from non-Hono code
 * (e.g., a module-level function that was handed a traceId explicitly).
 */
export interface TraceContext {
  var: { traceId: string };
}

/** Minimal fetch signature that orchestratorFetch requires. */
export type FetchFn = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

export interface OrchestratorFetchOpts {
  /** Override the orchestrator base URL. Defaults to process.env.ORCHESTRATOR_URL or http://localhost:7777. */
  baseUrl?: string;
  /** Override the fetch implementation (for tests). */
  fetch?: FetchFn;
}

export function resolveOrchestratorBase(opts: OrchestratorFetchOpts = {}): string {
  return opts.baseUrl ?? process.env.ORCHESTRATOR_URL ?? "http://localhost:7777";
}

/**
 * Fetch with X-Trace-ID automatically attached from c.var.traceId.
 * Always POSTs/GETs to the Go orchestrator; use only for orchestrator-bound calls.
 */
export async function orchestratorFetch(
  c: TraceContext,
  path: string,
  init: RequestInit = {},
  opts: OrchestratorFetchOpts = {},
): Promise<Response> {
  const base = resolveOrchestratorBase(opts);
  const headers = new Headers(init.headers);
  if (c?.var?.traceId) {
    headers.set("X-Trace-ID", c.var.traceId);
  }
  const f: FetchFn = opts.fetch ?? fetch;
  return f(`${base}${path}`, { ...init, headers });
}

/**
 * ingestToOrchestrator fire-and-forget logs a structured event to the Go orchestrator's
 * internal ingest endpoint. Never throws; ingest failures are silently dropped.
 */
export function ingestToOrchestrator(
  c: TraceContext,
  payload: {
    projectId?: string;
    tab?: "system" | "agent";
    level: "debug" | "info" | "warn" | "error";
    source: string;
    summary: string;
    detail?: Record<string, unknown>;
    eventType?: string;
    action?: string;
  },
  opts: OrchestratorFetchOpts = {},
): void {
  void orchestratorFetch(
    c,
    "/api/v1/internal/logs/ingest",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tab: "system", ...payload }),
      keepalive: true,
    },
    opts,
  ).catch(() => {
    // intentionally swallow — ingest must never break the bridge
  });
}
