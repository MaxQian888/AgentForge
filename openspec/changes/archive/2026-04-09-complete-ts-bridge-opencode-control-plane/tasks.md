## 1. OpenCode auth and pending interaction foundations

- [x] 1.1 Extend `src-bridge/src/opencode/transport.ts` with provider-auth start and callback completion helpers that preserve opaque upstream payloads.
- [x] 1.2 Add an OpenCode-specific pending interaction store for permission and provider-auth requests, including timeout and cleanup behavior.
- [x] 1.3 Wire OpenCode permission-request emission and `/bridge/permission-response/:request_id` forwarding against session-bound pending mappings instead of Claude-only callback state.

## 2. Paused session-backed control routes

- [x] 2.1 Add a resolver that locates OpenCode control context from either the active runtime pool or a persisted continuity snapshot.
- [x] 2.2 Update `messages`, `diff`, `revert`, and `unrevert` routes to use the resolver and return explicit continuity errors when the upstream session binding is unavailable.
- [x] 2.3 Update `command` and `shell` routes to use the same resolver so paused OpenCode tasks keep using the bound upstream session without forcing resume.
- [x] 2.4 Preserve the OpenCode session metadata needed for those control routes in pause/save/restore snapshot paths.

## 3. Runtime catalog truthfulness and provider-auth readiness

- [x] 3.1 Replace best-effort OpenCode catalog enrichment with explicit degraded diagnostics for agent, skill, and provider discovery failures.
- [x] 3.2 Publish provider-auth readiness and Bridge-startable auth support through the OpenCode runtime catalog entry and interaction capability metadata.
- [x] 3.3 Refresh OpenCode provider/catalog state after auth completion and relevant config updates so `/bridge/runtimes` reflects the latest truth.

## 4. Contract coverage, docs, and scoped verification

- [x] 4.1 Add transport and registry tests for provider-auth start/complete flows, degraded discovery diagnostics, and provider-auth catalog metadata.
- [x] 4.2 Add advanced route tests for paused-session `messages`, `diff`, `command`, `shell`, `revert`, and explicit continuity error responses.
- [x] 4.3 Add pending interaction tests for OpenCode permission/auth request-id mapping, expiry, and response forwarding.
- [x] 4.4 Update Bridge-facing documentation or README notes for the new OpenCode auth/control-plane behavior and paused-session semantics.
- [x] 4.5 Run scoped `src-bridge` verification (`bun test` and `bun x tsc --noEmit`) and record any remaining gaps before apply/archive.
