## 1. Auth Store — Extract Refresh Method

- [x] 1.1 Extract the refresh logic from `bootstrapSession()` in `lib/stores/auth-store.ts` into a standalone `refreshSession()` method that calls `POST /api/v1/auth/refresh`, stores the new tokens, and returns the new access token (or throws on failure)
- [x] 1.2 Update `bootstrapSession()` to call the new `refreshSession()` internally instead of inline refresh logic

## 2. API Client — 401 Interceptor with Retry

- [x] 2.1 Add a module-level `refreshPromise` variable in `lib/api-client.ts` (or a shared auth-refresh module) to serve as the coalescing lock for concurrent refresh attempts
- [x] 2.2 Implement a response interceptor in the API client that catches 401 responses, calls `refreshSession()` from the auth store, and retries the original request with the new access token
- [x] 2.3 Ensure the interceptor only retries once per original request — if the retried request still returns 401, propagate the error and clear the session
- [x] 2.4 Ensure non-401 errors (403, 500, etc.) pass through without triggering refresh

## 3. Concurrent Refresh Coalescing

- [x] 3.1 Implement the shared-promise pattern: when a refresh is triggered, store the promise in `refreshPromise`; subsequent 401 handlers await the existing promise instead of starting a new refresh
- [x] 3.2 Reset `refreshPromise` to `null` after the refresh completes (success or failure)

## 4. Session Cleanup on Refresh Failure

- [x] 4.1 Ensure that when `refreshSession()` fails (network error, 401 from refresh endpoint, invalid response), the auth store clears `accessToken`, `refreshToken`, and `user` and sets status to `unauthenticated`
- [x] 4.2 Verify that the protected layout redirects to `/login` when status transitions to `unauthenticated` mid-session

## 5. Testing

- [x] 5.1 Add unit tests for `refreshSession()` — success and failure paths
- [x] 5.2 Add unit tests for the 401 interceptor — single retry, concurrent coalescing, non-401 passthrough
