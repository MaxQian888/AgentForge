## 1. Frontend session contract and bootstrap

- [x] 1.1 Refactor `lib/stores/auth-store.ts` to use the canonical `accessToken`/`refreshToken`/`user` session shape plus explicit auth status instead of the current `token` + `isAuthenticated` contract.
- [x] 1.2 Introduce a backend URL resolver that auth flows can use in both web and Tauri modes, then route login, register, refresh, logout, and identity bootstrap through that resolver.
- [x] 1.3 Implement a single session bootstrap flow that validates stored auth with `/api/v1/users/me`, attempts one refresh when appropriate, and clears stale session state on failure.

## 2. Protected UI integration

- [x] 2.1 Update `app/(dashboard)/layout.tsx` and related auth-aware layout code to wait for auth bootstrap resolution before rendering or redirecting.
- [x] 2.2 Update header/logout and any directly dependent frontend auth consumers to use the new session API and local cleanup behavior.
- [x] 2.3 Update stores that currently read `useAuthStore().token` so authenticated requests continue to send the current access token after the auth-state refactor.

## 3. Backend auth hardening

- [x] 3.1 Update the auth backend so `GET /api/v1/users/me` loads the authoritative user profile from storage instead of returning claims-only identity data.
- [x] 3.2 Tighten refresh, logout, and JWT middleware behavior so cache-dependent revocation checks fail closed when Redis/token cache is unavailable.
- [x] 3.3 Adjust auth handler and route wiring as needed to preserve consistent HTTP error mapping and any auth-specific rate limiting required by the new flow.

## 4. Verification and documentation

- [x] 4.1 Add or update frontend tests covering canonical auth-state storage, bootstrap refresh recovery, stale-session clearing, and protected-layout redirect behavior.
- [x] 4.2 Add or update backend tests covering `/users/me`, refresh failure on cache unavailability, logout failure semantics, and protected-route rejection when blacklist verification cannot run.
- [x] 4.3 Update auth-related docs/config examples and run the scoped validation commands needed to confirm the new auth flow works end to end.
