import type { RuntimeDiagnostic } from "../types.js";

interface OpenCodeHealthResponse {
  healthy?: boolean;
  version?: string;
}

interface OpenCodeProviderResponse {
  all?: unknown[];
  connected?: unknown[];
}

interface OpenCodeProviderAuthResponse {
  [providerId: string]: unknown;
}

interface OpenCodeConfigProvidersResponse {
  default?: Record<string, string>;
  providers?: unknown[];
}

interface OpenCodeSessionResponse {
  id?: string;
}

interface OpenCodeNamedEntry {
  id?: string;
  name?: string;
  type?: string;
}

export interface OpenCodeProviderCatalog {
  connectedProviders: string[];
  availableProviders: string[];
  defaultModels: Record<string, string>;
  providerModels: Record<string, string[]>;
  authMethods: Record<string, string[]>;
}

export interface OpenCodeReadinessSelection {
  provider: string;
  model?: string;
}

export interface OpenCodeReadinessResult {
  ok: boolean;
  version?: string;
  diagnostics: RuntimeDiagnostic[];
  catalog?: OpenCodeProviderCatalog;
}

export interface OpenCodeSession {
  id: string;
}

export interface OpenCodeExecuteCapabilities {
  attachments: boolean;
  env: boolean;
  web_search: boolean;
  rollback: boolean;
}

export interface CreateOpenCodeSessionInput {
  title?: string;
  parentID?: string;
  provider?: string;
  model?: string;
  env?: Record<string, string>;
  web_search?: boolean;
}

export interface OpenCodePromptPart {
  type: string;
  [key: string]: unknown;
}

export interface OpenCodePromptAsyncInput {
  sessionId: string;
  prompt: string;
  provider: string;
  model?: string;
  parts?: OpenCodePromptPart[];
}

export interface OpenCodeServerEvent extends Record<string, unknown> {
  event: string;
  data: Record<string, unknown>;
}

interface OpenCodeTransportConfig {
  serverUrl?: string;
  username?: string;
  password?: string;
  fetchImpl: typeof fetch;
}

export interface CreateOpenCodeTransportOptions {
  serverUrl?: string;
  username?: string;
  password?: string;
  fetch?: typeof fetch;
  envLookup?: (name: string) => string | undefined;
}

export class OpenCodeTransport {
  constructor(private readonly config: OpenCodeTransportConfig) {}

  get serverUrl(): string | undefined {
    return this.config.serverUrl;
  }

  getExecuteCapabilities(): OpenCodeExecuteCapabilities {
    return {
      attachments: true,
      env: true,
      web_search: true,
      rollback: true,
    };
  }

  async getHealth(): Promise<{ healthy: boolean; version?: string }> {
    const response = await this.request("/global/health");
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok) {
      throw new Error(`OpenCode server health probe failed with status ${response.status}`);
    }

    const body = (await response.json()) as OpenCodeHealthResponse;
    return {
      healthy: body.healthy !== false,
      version: typeof body.version === "string" ? body.version : undefined,
    };
  }

  async getProviderCatalog(): Promise<OpenCodeProviderCatalog> {
    const providerResponse = await this.request("/provider");
    if (providerResponse.status === 401 || providerResponse.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!providerResponse.ok) {
      throw new Error(`OpenCode provider discovery failed with status ${providerResponse.status}`);
    }

    const configResponse = await this.request("/config/providers");
    if (configResponse.status === 401 || configResponse.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!configResponse.ok) {
      throw new Error(
        `OpenCode provider configuration discovery failed with status ${configResponse.status}`,
      );
    }

    const providerBody = (await providerResponse.json()) as OpenCodeProviderResponse;
    const configBody = (await configResponse.json()) as OpenCodeConfigProvidersResponse;
    const authMethods = await this.getProviderAuthMethods();

    const availableProviders = extractProviderIDs(providerBody.all);
    const connectedProviders = extractStringArray(providerBody.connected);
    const providerModels = extractProviderModels(configBody.providers);

    return {
      connectedProviders,
      availableProviders,
      defaultModels: isStringRecord(configBody.default) ? configBody.default : {},
      providerModels,
      authMethods,
    };
  }

  async getProviderAuthMethods(): Promise<Record<string, string[]>> {
    const response = await this.request("/provider/auth");
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (response.status === 404 || response.status === 405 || response.status === 501) {
      return {};
    }
    if (!response.ok) {
      throw new Error(`OpenCode provider authentication discovery failed with status ${response.status}`);
    }

    const body = (await response.json()) as OpenCodeProviderAuthResponse;
    if (!body || typeof body !== "object") {
      return {};
    }

    return Object.fromEntries(
      Object.entries(body).map(([providerId, methods]) => [
        providerId,
        extractNamedEntries(methods).map((value) => value.toLowerCase()),
      ]),
    );
  }

  async startProviderOAuth(
    provider: string,
    payload: Record<string, unknown> = {},
  ): Promise<unknown> {
    const response = await this.request(`/provider/${provider}/oauth/authorize`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return this.readJsonResponse(
      response,
      `OpenCode provider ${provider} oauth authorize failed`,
    );
  }

  async completeProviderOAuth(
    provider: string,
    payload: Record<string, unknown>,
  ): Promise<unknown> {
    const response = await this.request(`/provider/${provider}/oauth/callback`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return this.readJsonResponse(
      response,
      `OpenCode provider ${provider} oauth callback failed`,
    );
  }

  async checkReadiness(
    selection: OpenCodeReadinessSelection,
  ): Promise<OpenCodeReadinessResult> {
    if (!this.config.serverUrl) {
      return {
        ok: false,
        diagnostics: [
          {
            code: "missing_server_url",
            message: "Missing required OpenCode server URL: OPENCODE_SERVER_URL",
            blocking: true,
          },
        ],
      };
    }

    try {
      const health = await this.getHealth();
      const catalog = await this.getProviderCatalog();
      const diagnostics: RuntimeDiagnostic[] = [];
      const provider = selection.provider.trim().toLowerCase();
      const model = selection.model?.trim();

      if (!catalog.availableProviders.includes(provider)) {
        diagnostics.push({
          code: "provider_unavailable",
          message: `OpenCode provider ${provider} is not available from the configured server`,
          blocking: true,
        });
      } else if (!catalog.connectedProviders.includes(provider)) {
        diagnostics.push({
          code: "provider_unavailable",
          message: `OpenCode provider ${provider} is not connected on the configured server`,
          blocking: true,
        });
      }

      if (model) {
        const providerModels = catalog.providerModels[provider] ?? [];
        const defaultModel = catalog.defaultModels[provider];
        if (!providerModels.includes(model) && defaultModel !== model) {
          diagnostics.push({
            code: "model_unavailable",
            message: `OpenCode model ${model} is not available for provider ${provider}`,
            blocking: true,
          });
        }
      }

      return {
        ok: diagnostics.length === 0,
        version: health.version,
        diagnostics,
        catalog,
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (message.includes("authentication failed")) {
        return {
          ok: false,
          diagnostics: [
            {
              code: "authentication_failed",
              message: "OpenCode server authentication failed",
              blocking: true,
            },
          ],
        };
      }

      return {
        ok: false,
        diagnostics: [
          {
            code: "server_unreachable",
            message: `OpenCode server ${this.config.serverUrl} is unreachable: ${message}`,
            blocking: true,
          },
        ],
      };
    }
  }

  async createSession(input: CreateOpenCodeSessionInput = {}): Promise<OpenCodeSession> {
    const response = await this.request("/session", {
      method: "POST",
      body: JSON.stringify(input),
    });
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok) {
      throw new Error(`OpenCode session creation failed with status ${response.status}`);
    }

    const body = (await response.json()) as OpenCodeSessionResponse;
    if (!body.id) {
      throw new Error("OpenCode session creation returned no session id");
    }
    return { id: body.id };
  }

  async sendPromptAsync(input: OpenCodePromptAsyncInput): Promise<void> {
    const response = await this.request(`/session/${input.sessionId}/prompt_async`, {
      method: "POST",
      body: JSON.stringify({
        model: input.model,
        provider: input.provider,
        parts:
          input.parts && input.parts.length > 0
            ? input.parts
            : [
                {
                  type: "text",
                  text: input.prompt,
                },
              ],
      }),
    });
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (response.status !== 204 && !response.ok) {
      throw new Error(`OpenCode prompt_async failed with status ${response.status}`);
    }
  }

  async abortSession(sessionId: string): Promise<boolean> {
    const response = await this.request(`/session/${sessionId}/abort`, {
      method: "POST",
    });
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok) {
      throw new Error(`OpenCode session abort failed with status ${response.status}`);
    }

    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      return true;
    }

    const body = (await response.json()) as boolean;
    return body;
  }

  async forkSession(sessionId: string, messageId?: string): Promise<OpenCodeSession> {
    const response = await this.request(`/session/${sessionId}/fork`, {
      method: "POST",
      body: JSON.stringify(messageId ? { messageID: messageId } : {}),
    });
    return this.readSessionResponse(response, "OpenCode session fork failed");
  }

  async revertMessage(sessionId: string, messageId: string): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/revert`, {
      method: "POST",
      body: JSON.stringify({ messageID: messageId }),
    });
    return this.readJsonResponse(response, "OpenCode session revert failed");
  }

  async unrevertMessages(sessionId: string): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/unrevert`, {
      method: "POST",
    });
    return this.readJsonResponse(response, "OpenCode session unrevert failed");
  }

  async getDiff(sessionId: string): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/diff`);
    return this.readJsonResponse(response, "OpenCode session diff lookup failed");
  }

  async getTodos(sessionId: string): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/todo`);
    return this.readJsonResponse(response, "OpenCode session todo lookup failed");
  }

  async getMessages(sessionId: string): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/message`);
    return this.readJsonResponse(response, "OpenCode session message lookup failed");
  }

  async executeCommand(sessionId: string, command: string, args?: string): Promise<unknown> {
    const normalized = command.startsWith("/") ? command.slice(1) : command;
    const response = await this.request(`/session/${sessionId}/command`, {
      method: "POST",
      body: JSON.stringify({
        name: normalized,
        args: args ? [args] : undefined,
      }),
    });
    return this.readJsonResponse(response, "OpenCode session command failed");
  }

  async executeShell(
    sessionId: string,
    command: string,
    agent?: string,
    model?: string,
  ): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/shell`, {
      method: "POST",
      body: JSON.stringify({
        agent: agent ?? "general",
        command,
        model,
      }),
    });
    return this.readJsonResponse(response, "OpenCode session shell command failed");
  }

  async respondToPermission(
    sessionId: string,
    permissionId: string,
    allow: boolean,
  ): Promise<unknown> {
    const response = await this.request(`/session/${sessionId}/permissions/${permissionId}`, {
      method: "POST",
      body: JSON.stringify({ allow }),
    });
    return this.readJsonResponse(response, "OpenCode permission response failed");
  }

  async getAgents(): Promise<string[]> {
    const response = await this.request("/agent");
    const body = await this.readJsonResponse(response, "OpenCode agent discovery failed");
    return extractNamedEntries(body);
  }

  async getSkills(): Promise<string[]> {
    const response = await this.request("/skill");
    const body = await this.readJsonResponse(response, "OpenCode skill discovery failed");
    return extractNamedEntries(body);
  }

  async updateConfig(config: Record<string, unknown>): Promise<unknown> {
    const response = await this.request("/config", {
      method: "PATCH",
      body: JSON.stringify(config),
    });
    return this.readJsonResponse(response, "OpenCode config update failed");
  }

  async *streamEvents(abortSignal?: AbortSignal): AsyncGenerator<OpenCodeServerEvent, void> {
    const response = await this.request("/event", {
      signal: abortSignal,
      headers: {
        accept: "text/event-stream",
      },
    });
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok || !response.body) {
      throw new Error(`OpenCode event stream failed with status ${response.status}`);
    }

    let eventName = "message";
    let dataBuffer = "";
    for await (const line of readLines(response.body)) {
      if (!line.length) {
        if (dataBuffer.trim()) {
          const parsed = safeParseJSON(dataBuffer);
          if (parsed) {
            yield {
              event: eventName,
              data: parsed,
            };
          }
        }
        eventName = "message";
        dataBuffer = "";
        continue;
      }

      if (line.startsWith("event:")) {
        eventName = line.slice("event:".length).trim() || "message";
        continue;
      }
      if (line.startsWith("data:")) {
        const chunk = line.slice("data:".length).trim();
        dataBuffer = dataBuffer ? `${dataBuffer}\n${chunk}` : chunk;
      }
    }
  }

  private request(pathname: string, init?: RequestInit): Promise<Response> {
    if (!this.config.serverUrl) {
      throw new Error("OpenCode server URL is not configured");
    }

    const url = new URL(pathname, this.config.serverUrl).toString();
    const headers = new Headers(init?.headers);
    const authHeaders = buildHeaders(this.config.username, this.config.password);
    if (authHeaders) {
      for (const [key, value] of Object.entries(authHeaders)) {
        headers.set(key, value);
      }
    }
    if (init?.body && !headers.has("content-type")) {
      headers.set("content-type", "application/json");
    }
    return this.config.fetchImpl(url, {
      ...init,
      headers,
    });
  }

  private async readSessionResponse(response: Response, message: string): Promise<OpenCodeSession> {
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok) {
      throw new Error(`${message} with status ${response.status}`);
    }

    const body = (await response.json()) as OpenCodeSessionResponse;
    if (!body.id) {
      throw new Error(`${message}: response returned no session id`);
    }
    return { id: body.id };
  }

  private async readJsonResponse(response: Response, message: string): Promise<unknown> {
    if (response.status === 401 || response.status === 403) {
      throw new Error("OpenCode server authentication failed");
    }
    if (!response.ok) {
      throw new Error(`${message} with status ${response.status}`);
    }

    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      return true;
    }
    return response.json();
  }
}

export function createOpenCodeTransport(
  options: CreateOpenCodeTransportOptions = {},
): OpenCodeTransport {
  const envLookup = options.envLookup ?? ((name: string) => process.env[name]);
  const password = readEnvConfig(options.password ?? envLookup("OPENCODE_SERVER_PASSWORD"));
  const username =
    readEnvConfig(options.username ?? envLookup("OPENCODE_SERVER_USERNAME")) ??
    (password ? "opencode" : undefined);

  return new OpenCodeTransport({
    serverUrl: normalizeServerUrl(
      readEnvConfig(options.serverUrl ?? envLookup("OPENCODE_SERVER_URL")),
    ),
    username,
    password,
    fetchImpl: options.fetch ?? globalThis.fetch,
  });
}

function buildHeaders(username?: string, password?: string): HeadersInit | undefined {
  if (!username || !password) {
    return undefined;
  }

  const token = Buffer.from(`${username}:${password}`).toString("base64");
  return {
    authorization: `Basic ${token}`,
  };
}

function normalizeServerUrl(serverUrl: string | undefined): string | undefined {
  if (!serverUrl) {
    return undefined;
  }

  const normalized = serverUrl.trim();
  if (!normalized) {
    return undefined;
  }

  return normalized.endsWith("/") ? normalized : `${normalized}/`;
}

function readEnvConfig(value: string | undefined): string | undefined {
  const normalized = value?.trim();
  return normalized ? normalized : undefined;
}

function extractProviderIDs(input: unknown[] | undefined): string[] {
  if (!Array.isArray(input)) {
    return [];
  }

  return input
    .map((entry) => {
      if (typeof entry === "string") {
        return entry;
      }
      if (entry && typeof entry === "object" && typeof (entry as { id?: unknown }).id === "string") {
        return (entry as { id: string }).id;
      }
      return undefined;
    })
    .filter((value): value is string => Boolean(value));
}

function extractStringArray(input: unknown): string[] {
  if (!Array.isArray(input)) {
    return [];
  }

  return input.filter((value): value is string => typeof value === "string");
}

function extractProviderModels(input: unknown[] | undefined): Record<string, string[]> {
  if (!Array.isArray(input)) {
    return {};
  }

  const models: Record<string, string[]> = {};
  for (const entry of input) {
    if (!entry || typeof entry !== "object") {
      continue;
    }

    const providerId =
      typeof (entry as { id?: unknown }).id === "string"
        ? (entry as { id: string }).id
        : undefined;
    if (!providerId) {
      continue;
    }

    const rawModels = (entry as { models?: unknown }).models;
    if (!Array.isArray(rawModels)) {
      models[providerId] = [];
      continue;
    }

    models[providerId] = rawModels
      .map((model) => {
        if (typeof model === "string") {
          return model;
        }
        if (model && typeof model === "object" && typeof (model as { id?: unknown }).id === "string") {
          return (model as { id: string }).id;
        }
        return undefined;
      })
      .filter((value): value is string => Boolean(value));
  }

  return models;
}

function extractNamedEntries(input: unknown): string[] {
  if (!Array.isArray(input)) {
    return [];
  }

  return input
    .map((entry) => {
      if (typeof entry === "string") {
        return entry;
      }
      if (!entry || typeof entry !== "object") {
        return undefined;
      }
      const namedEntry = entry as OpenCodeNamedEntry;
      return typeof namedEntry.id === "string"
        ? namedEntry.id
        : typeof namedEntry.name === "string"
          ? namedEntry.name
          : typeof namedEntry.type === "string"
            ? namedEntry.type
          : undefined;
    })
    .filter((value): value is string => Boolean(value));
}

function isStringRecord(value: unknown): value is Record<string, string> {
  if (!value || typeof value !== "object") {
    return false;
  }

  return Object.values(value).every((entry) => typeof entry === "string");
}

async function* readLines(
  stream: ReadableStream<Uint8Array>,
): AsyncGenerator<string, void, undefined> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffered = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      buffered += decoder.decode(value, { stream: true });
      const lines = buffered.split(/\r?\n/);
      buffered = lines.pop() ?? "";
      for (const line of lines) {
        yield line;
      }
    }
  } finally {
    reader.releaseLock();
  }

  const tail = buffered + decoder.decode();
  if (tail.length > 0) {
    yield tail;
  }
}

function safeParseJSON(input: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(input);
    return typeof parsed === "object" && parsed !== null
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}
