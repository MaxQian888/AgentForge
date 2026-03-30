import { randomUUID } from "node:crypto";

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

interface HookCallbackManagerOptions {
  fetchImpl?: FetchLike;
  idGenerator?: () => string;
}

interface PendingCallback {
  timer: ReturnType<typeof setTimeout>;
  resolve: (value: unknown) => void;
  reject: (error: Error) => void;
}

export interface HookCallbackRegistration {
  requestId: string;
  response: Promise<unknown>;
}

export interface RegisterHookCallbackInput {
  callbackUrl: string;
  payload: Record<string, unknown>;
  timeoutMs?: number;
}

export class HookCallbackManager {
  private readonly fetchImpl: FetchLike;
  private readonly idGenerator: () => string;
  private readonly pending = new Map<string, PendingCallback>();

  constructor(options: HookCallbackManagerOptions = {}) {
    this.fetchImpl = options.fetchImpl ?? globalThis.fetch;
    this.idGenerator = options.idGenerator ?? (() => randomUUID());
  }

  async register(input: RegisterHookCallbackInput): Promise<HookCallbackRegistration> {
    const requestId = this.idGenerator();
    const timeoutMs = input.timeoutMs && input.timeoutMs > 0 ? input.timeoutMs : 5000;
    let resolvePending!: (value: unknown) => void;
    let rejectPending!: (error: Error) => void;

    const response = new Promise<unknown>((resolve, reject) => {
      resolvePending = resolve;
      rejectPending = reject;
    });

    const timer = setTimeout(() => {
      if (!this.pending.delete(requestId)) {
        return;
      }
      rejectPending(new Error(`Hook callback ${requestId} timed out after ${timeoutMs}ms`));
    }, timeoutMs);

    this.pending.set(requestId, {
      timer,
      resolve: resolvePending,
      reject: rejectPending,
    });

    try {
      const registrationResponse = await this.fetchImpl(input.callbackUrl, {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          ...input.payload,
          request_id: requestId,
        }),
      });

      if (!registrationResponse.ok) {
        throw new Error(
          `Hook callback registration failed with status ${registrationResponse.status}`,
        );
      }
    } catch (error) {
      const normalized = error instanceof Error ? error : new Error(String(error));
      const pending = this.pending.get(requestId);
      if (pending) {
        this.pending.delete(requestId);
        clearTimeout(pending.timer);
      }
      throw normalized;
    }

    return {
      requestId,
      response,
    };
  }

  resolve(requestId: string, payload: unknown): boolean {
    const pending = this.pending.get(requestId);
    if (!pending) {
      return false;
    }

    this.pending.delete(requestId);
    clearTimeout(pending.timer);
    pending.resolve(payload);
    return true;
  }

  reject(requestId: string, error: Error): boolean {
    const pending = this.pending.get(requestId);
    if (!pending) {
      return false;
    }

    this.pending.delete(requestId);
    clearTimeout(pending.timer);
    pending.reject(error);
    return true;
  }
}
