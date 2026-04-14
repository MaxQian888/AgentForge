## Context

The frontend auth flow currently refreshes the access token only during `bootstrapSession()` — called once on page load. The access token TTL is 15 minutes. Once it expires during an active session, all authenticated API calls return 401 and the user must reload the page. The backend refresh endpoint (`POST /api/v1/auth/refresh`) is fully functional and returns rotated token pairs; the gap is entirely on the frontend.

The API client (`lib/api-client.ts`) is a thin fetch wrapper with no response interceptors. The auth store (`lib/stores/auth-store.ts`) exposes `bootstrapSession()` but has no standalone refresh method callable during normal operation.

## Goals / Non-Goals

**Goals:**
- Transparently refresh the access token when a 401 is encountered during an active session
- Coalesce concurrent refresh attempts so only one refresh request is in-flight at a time
- Maintain fail-closed behavior: if refresh fails, clear session and redirect to login

**Non-Goals:**
- Backend changes (the refresh endpoint already works correctly)
- Proactive near-expiry refresh (the 401 interceptor retry latency is imperceptible; added complexity not justified)
- Sliding session / keep-alive pings
- Offline/background tab token refresh
- Changing token TTLs

## Decisions

### 1. 401-interceptor with automatic retry in the API client

**Choice**: Add a response interceptor layer to `api-client.ts` that catches 401 responses, triggers a token refresh, and retries the original request with the new token.

**Why over alternatives:**
- *Alternative: Global fetch override* — too broad, affects non-auth requests and third-party calls.
- *Alternative: Per-call retry in each store* — duplicates logic across 30+ stores, error-prone.
- The API client is the single chokepoint for all authenticated backend calls, making it the natural place for this.

### 2. Refresh lock via a shared Promise

**Choice**: Use a module-level `refreshPromise: Promise | null` variable. When a 401 triggers refresh, set `refreshPromise` to the in-flight refresh call. Subsequent 401s await the same promise instead of issuing parallel refresh requests. Reset to `null` on completion.

**Why**: Simple, no external dependencies, prevents the token-rotation race where two concurrent refreshes invalidate each other (since the backend deletes the old refresh token on use).

### 3. Extract `refreshSession()` from bootstrap into its own method

**Choice**: Factor the refresh logic out of `bootstrapSession()` into a standalone `refreshSession()` method on the auth store. `bootstrapSession()` calls it internally. The API client interceptor calls it via `useAuthStore.getState().refreshSession()`.

**Why**: Avoids duplicating refresh logic. The auth store remains the single owner of token state.

## Risks / Trade-offs

- **[Risk] Infinite retry loop if backend returns 401 on refresh** → Mitigation: The interceptor only retries once per original request. If the retried request still returns 401, it propagates the error and triggers logout.
- **[Risk] Brief retry latency on first expired request** → Accepted: The refresh + retry round-trip is <500ms, imperceptible to users. Not worth the complexity of proactive refresh.
