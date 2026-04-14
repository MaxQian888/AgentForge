import { describe, expect, test } from "bun:test";
import { OpenCodePendingInteractionStore } from "./pending-interactions.js";

describe("OpenCodePendingInteractionStore", () => {
  test("maps permission requests to request ids and resolves allow or deny responses", () => {
    const store = new OpenCodePendingInteractionStore({
      idGenerator: () => "permission-request-1",
      now: () => 100,
      ttlMs: 5_000,
    });

    const created = store.createPermissionRequest({
      sessionId: "opencode-session-123",
      permissionId: "perm-42",
      toolName: "Read",
    });
    const resolved = store.resolvePermissionResponse(created.requestId, {
      decision: "allow",
      reason: "approved",
    });

    expect(created).toEqual({ requestId: "permission-request-1" });
    expect(resolved).toEqual({
      sessionId: "opencode-session-123",
      permissionId: "perm-42",
      allow: true,
      reason: "approved",
    });
    expect(
      store.resolvePermissionResponse(created.requestId, {
        decision: "deny",
      }),
    ).toBeNull();
  });

  test("expires pending permission and provider-auth requests after their ttl", () => {
    let now = 100;
    const store = new OpenCodePendingInteractionStore({
      idGenerator: (() => {
        let counter = 0;
        return () => `request-${++counter}`;
      })(),
      now: () => now,
      ttlMs: 10,
    });

    const permission = store.createPermissionRequest({
      sessionId: "opencode-session-123",
      permissionId: "perm-42",
    });
    const providerAuth = store.createProviderAuthRequest({
      provider: "anthropic",
    });

    now = 200;

    expect(
      store.resolvePermissionResponse(permission.requestId, {
        decision: "allow",
      }),
    ).toBeNull();
    expect(store.consumeProviderAuthRequest(providerAuth.requestId)).toBeNull();
  });

  test("consumes provider-auth requests exactly once", () => {
    const store = new OpenCodePendingInteractionStore({
      idGenerator: () => "provider-auth-1",
      now: () => 100,
      ttlMs: 5_000,
    });

    const created = store.createProviderAuthRequest({
      provider: "anthropic",
    });

    expect(created).toEqual({ requestId: "provider-auth-1" });
    expect(store.consumeProviderAuthRequest(created.requestId)).toEqual({
      provider: "anthropic",
    });
    expect(store.consumeProviderAuthRequest(created.requestId)).toBeNull();
  });
});
