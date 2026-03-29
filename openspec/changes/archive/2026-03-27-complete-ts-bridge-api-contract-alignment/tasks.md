## 1. Contract Inventory And Spec Mapping

- [x] 1.1 Audit `src-bridge/src/server.ts`, `src-go/internal/bridge/client.go`, and existing bridge tests to list every live canonical route and every remaining compatibility alias.
- [x] 1.2 Map the audited route surface against `docs/PRD.md` and related design docs, and identify every place that still presents `/api/*`, `/agent/*`, or gRPC as the live TS Bridge contract.

## 2. Bridge Route Surface Alignment

- [x] 2.1 Update `src-bridge/src/server.ts` route registration and inline comments so canonical `/bridge/*` routes are explicit and compatibility aliases are clearly secondary.
- [x] 2.2 Ensure compatibility aliases delegate to the same handlers and schema validation as canonical routes for execute, runtime lifecycle, and lightweight AI operations.
- [x] 2.3 Add or update focused bridge tests that verify canonical routes, alias parity, and validation equivalence for the affected operations.

## 3. Go Caller Alignment

- [x] 3.1 Keep `src-go/internal/bridge/client.go` and related adapters on the canonical `/bridge/*` contract for execute, status, cancel, pause, resume, runtimes, decomposition, classification, and generation flows.
- [x] 3.2 Update Go-side bridge client tests and any call-site assertions so canonical routes are the primary expected behavior and no new historical route families are introduced.

## 4. Documentation And Verification

- [x] 4.1 Rewrite TS Bridge live-contract sections in `docs/PRD.md` and related architecture/design docs to identify HTTP + WebSocket plus `/bridge/*` as the current implementation truth, while marking retained gRPC/proto content as historical or reference-only.
- [x] 4.2 Update any README, runbook, or developer-facing examples that still teach non-canonical TS Bridge routes.
- [x] 4.3 Run focused verification for the touched bridge and Go tests, plus a targeted documentation search, and confirm the repo no longer presents historical routes as the primary TS Bridge contract.
