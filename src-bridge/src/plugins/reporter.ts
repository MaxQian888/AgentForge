import type { PluginRuntimeReporter, PluginRuntimeUpdate } from "./types.js";

class NoopPluginRuntimeReporter implements PluginRuntimeReporter {
  async report(update: PluginRuntimeUpdate): Promise<void> {
    void update;
  }
}

export class HttpPluginRuntimeReporter implements PluginRuntimeReporter {
  constructor(private readonly endpoint: string) {}

  async report(update: PluginRuntimeUpdate): Promise<void> {
    const response = await fetch(this.endpoint, {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify(update),
    });

    if (!response.ok) {
      throw new Error(`Plugin runtime report failed with status ${response.status}`);
    }
  }
}

export function createDefaultPluginRuntimeReporter(): PluginRuntimeReporter {
  const endpoint = process.env.GO_PLUGIN_RUNTIME_SYNC_URL;
  if (!endpoint) {
    return new NoopPluginRuntimeReporter();
  }
  return new HttpPluginRuntimeReporter(endpoint);
}
