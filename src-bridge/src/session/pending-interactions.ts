import { randomUUID } from "node:crypto";

export interface OpenCodePermissionRequestInput {
  sessionId: string;
  permissionId: string;
  toolName?: string;
  context?: unknown;
  ttlMs?: number;
}

export interface OpenCodeResolvedPermissionResponse {
  sessionId: string;
  permissionId: string;
  allow: boolean;
  reason?: string;
}

export interface OpenCodeProviderAuthRequestInput {
  provider: string;
  ttlMs?: number;
}

interface PendingPermissionRequest {
  sessionId: string;
  permissionId: string;
  expiresAt: number;
}

interface PendingProviderAuthRequest {
  provider: string;
  expiresAt: number;
}

interface OpenCodePendingInteractionStoreOptions {
  idGenerator?: () => string;
  now?: () => number;
  ttlMs?: number;
}

export class OpenCodePendingInteractionStore {
  private readonly idGenerator: () => string;
  private readonly now: () => number;
  private readonly ttlMs: number;
  private readonly permissionRequests = new Map<string, PendingPermissionRequest>();
  private readonly providerAuthRequests = new Map<string, PendingProviderAuthRequest>();

  constructor(options: OpenCodePendingInteractionStoreOptions = {}) {
    this.idGenerator = options.idGenerator ?? (() => randomUUID());
    this.now = options.now ?? Date.now;
    this.ttlMs = options.ttlMs ?? 5 * 60 * 1000;
  }

  createPermissionRequest(input: OpenCodePermissionRequestInput): { requestId: string } {
    this.cleanupExpired();
    const requestId = this.idGenerator();
    this.permissionRequests.set(requestId, {
      sessionId: input.sessionId,
      permissionId: input.permissionId,
      expiresAt: this.now() + (input.ttlMs ?? this.ttlMs),
    });
    return { requestId };
  }

  getPermissionRequest(
    requestId: string,
  ): { sessionId: string; permissionId: string } | null {
    this.cleanupExpired();
    const pending = this.permissionRequests.get(requestId);
    if (!pending) {
      return null;
    }
    return {
      sessionId: pending.sessionId,
      permissionId: pending.permissionId,
    };
  }

  consumePermissionRequest(requestId: string): boolean {
    this.cleanupExpired();
    return this.permissionRequests.delete(requestId);
  }

  resolvePermissionResponse(
    requestId: string,
    payload: { decision: "allow" | "deny"; reason?: string },
  ): OpenCodeResolvedPermissionResponse | null {
    this.cleanupExpired();
    const pending = this.permissionRequests.get(requestId);
    if (!pending) {
      return null;
    }
    this.permissionRequests.delete(requestId);
    return {
      sessionId: pending.sessionId,
      permissionId: pending.permissionId,
      allow: payload.decision === "allow",
      reason: payload.reason,
    };
  }

  createProviderAuthRequest(
    input: OpenCodeProviderAuthRequestInput,
  ): { requestId: string } {
    this.cleanupExpired();
    const requestId = this.idGenerator();
    this.providerAuthRequests.set(requestId, {
      provider: input.provider,
      expiresAt: this.now() + (input.ttlMs ?? this.ttlMs),
    });
    return { requestId };
  }

  consumeProviderAuthRequest(
    requestId: string,
  ): { provider: string } | null {
    this.cleanupExpired();
    const pending = this.providerAuthRequests.get(requestId);
    if (!pending) {
      return null;
    }
    this.providerAuthRequests.delete(requestId);
    return {
      provider: pending.provider,
    };
  }

  private cleanupExpired(): void {
    const now = this.now();
    for (const [requestId, pending] of this.permissionRequests) {
      if (pending.expiresAt <= now) {
        this.permissionRequests.delete(requestId);
      }
    }
    for (const [requestId, pending] of this.providerAuthRequests) {
      if (pending.expiresAt <= now) {
        this.providerAuthRequests.delete(requestId);
      }
    }
  }
}
