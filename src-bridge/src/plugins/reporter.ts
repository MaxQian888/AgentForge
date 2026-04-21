import type { PluginRuntimeReporter, PluginRuntimeUpdate } from "./types.js";
import { orchestratorFetch, type TraceContext } from "../lib/orchestrator-fetch.js";

/** A zero-trace context used when no Hono request context is in scope. */
const NO_TRACE: TraceContext = { var: { traceId: "" } };

class NoopPluginRuntimeReporter implements PluginRuntimeReporter {
  async report(update: PluginRuntimeUpdate): Promise<void> {
    void update;
  }
}

export class HttpPluginRuntimeReporter implements PluginRuntimeReporter {
  constructor(
    private readonly endpoint: string,
    private readonly traceCtx: TraceContext = NO_TRACE,
  ) {}

  async report(update: PluginRuntimeUpdate): Promise<void> {
    const url = new URL(this.endpoint);
    const path = url.pathname + url.search;
    const response = await orchestratorFetch(
      this.traceCtx,
      path,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(update),
      },
      { baseUrl: url.origin },
    );

    if (!response.ok) {
      throw new Error(`Plugin runtime report failed with status ${response.status}`);
    }
  }
}

export function createDefaultPluginRuntimeReporter(
  traceCtx: TraceContext = NO_TRACE,
): PluginRuntimeReporter {
  const endpoint = process.env.GO_PLUGIN_RUNTIME_SYNC_URL;
  if (!endpoint) {
    return new NoopPluginRuntimeReporter();
  }
  return new HttpPluginRuntimeReporter(endpoint, traceCtx);
}
