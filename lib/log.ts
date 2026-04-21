export type Level = "debug" | "info" | "warn" | "error";

interface Entry {
  tab: "system";
  level: Level;
  source: "frontend";
  summary: string;
  detail?: Record<string, unknown>;
  ts: number;
}

export interface BrowserLoggerOpts {
  fetch?: typeof fetch;
  flushMs?: number;
  bufferSize?: number;
  traceId: () => string;
  endpoint?: string;
}

export interface BrowserLogger {
  debug(summary: string, detail?: Record<string, unknown>): void;
  info(summary: string, detail?: Record<string, unknown>): void;
  warn(summary: string, detail?: Record<string, unknown>): void;
  error(summary: string, detail?: Record<string, unknown>): void;
}

const INGEST_LEVELS: ReadonlySet<Level> = new Set(["warn", "error"]);

export function createBrowserLogger(opts: BrowserLoggerOpts): BrowserLogger {
  const f = opts.fetch ?? fetch.bind(globalThis);
  const flushMs = opts.flushMs ?? 1000;
  const cap = opts.bufferSize ?? 20;
  const endpoint = opts.endpoint ?? "/api/v1/internal/logs/ingest";
  const buffer: Entry[] = [];
  let timer: ReturnType<typeof setTimeout> | null = null;

  function schedule() {
    if (timer !== null) return;
    timer = setTimeout(flush, flushMs);
  }

  async function flush() {
    timer = null;
    if (buffer.length === 0) return;
    const batch = buffer.splice(0, buffer.length);
    try {
      await f(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Trace-ID": opts.traceId(),
        },
        body: JSON.stringify(batch),
        keepalive: true,
      });
    } catch {
      // never block UI on ingest failure
    }
  }

  function push(level: Level, summary: string, detail?: Record<string, unknown>) {
    // Always mirror to console (dev ergonomics). Skip ingest for debug/info.
    const consoleFn: (...args: unknown[]) => void =
      (console as unknown as Record<Level, (...args: unknown[]) => void>)[level] ??
      (console as unknown as { log: (...args: unknown[]) => void }).log;
    consoleFn(`[${level}]`, summary, detail ?? "");

    if (!INGEST_LEVELS.has(level)) {
      return;
    }
    buffer.push({ tab: "system", level, source: "frontend", summary, detail, ts: Date.now() });
    if (buffer.length >= cap) {
      void flush();
    } else {
      schedule();
    }
  }

  return {
    debug: (s, d) => push("debug", s, d),
    info:  (s, d) => push("info",  s, d),
    warn:  (s, d) => push("warn",  s, d),
    error: (s, d) => push("error", s, d),
  };
}

let current: BrowserLogger | null = null;
let currentTrace = "";

/** Shared singleton logger. First call lazily initializes; trace id is read at each flush. */
export function log(): BrowserLogger {
  if (!current) {
    current = createBrowserLogger({ traceId: () => currentTrace });
  }
  return current;
}

/** Rotate the active trace_id (called by the fetch interceptor or page root). */
export function setTraceId(id: string): void {
  currentTrace = id;
}

export function getTraceId(): string {
  return currentTrace;
}
