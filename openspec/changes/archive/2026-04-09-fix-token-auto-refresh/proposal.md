## Why

The current auth implementation only refreshes the access token during page load (bootstrap). When a user stays active in the app beyond the 15-minute access token TTL, API calls start failing with 401 errors because there is no mechanism to automatically refresh the expired token during an active session. This forces users to reload the page or re-login, breaking their workflow.

## What Changes

- Add a transparent token refresh interceptor to the API client that detects 401 responses, refreshes the token, and retries the original request automatically.
- Implement a token refresh lock to prevent concurrent refresh requests when multiple API calls fail simultaneously.
- Update the auth store to expose a `refreshSession()` method usable outside of bootstrap.
- Ensure failed auto-refresh (e.g., expired refresh token) triggers logout and redirect to login.

## Capabilities

### New Capabilities

_(none — this is a completion of existing auth-session-management capability)_

### Modified Capabilities

- `auth-session-management`: Add requirements for automatic in-session token refresh when the access token expires during active use, including retry-on-401 and concurrent refresh coalescing.

## Impact

- **Frontend**: `lib/api-client.ts` (add 401 interceptor + retry logic), `lib/stores/auth-store.ts` (expose refresh method)
- **Backend**: No changes required — the existing `POST /api/v1/auth/refresh` endpoint already supports the needed flow.
- **Security**: Refresh lock prevents token race conditions. Failed refresh still clears session (fail-closed).
