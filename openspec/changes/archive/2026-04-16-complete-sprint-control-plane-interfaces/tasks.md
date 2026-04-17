## 1. Canonicalize sprint control-plane queries

- [x] 1.1 Extend sprint repository, models, and handlers to honor sprint list filters and expose a canonical project-scoped current sprint lookup with explicit zero/one/many active outcomes.
- [x] 1.2 Move sprint burndown and budget detail reads onto project-scoped routes with sprint-to-project scope checks, and update backend route wiring plus targeted handler tests.
- [x] 1.3 Enforce the single-active-sprint invariant during sprint status updates and cover activation conflicts with focused Go tests.

## 2. Wire the web sprint workspace to the canonical contract

- [x] 2.1 Update `lib/stores/sprint-store.ts` and related API helpers to consume the canonical current/detail routes and preserve deterministic error state for sprint detail reads.
- [x] 2.2 Add `?project=<id>&sprint=<sid>` selection seeding to `app/(dashboard)/sprints/page.tsx` and any supporting route helpers, with graceful fallback when the seed is invalid or stale.
- [x] 2.3 Surface active-sprint conflict errors inline in the sprint edit flow and refresh sprint workspace tests for explicit selection, fallback, and conflict handling.

## 3. Migrate IM consumers and verify the contract

- [x] 3.1 Update `src-im-bridge/client/agentforge.go` and `src-im-bridge/commands/sprint.go` to use the canonical current sprint and project-scoped detail endpoints instead of list-order assumptions.
- [x] 3.2 Refresh IM bridge tests, backend route tests, and any sprint-related docs or fixtures that still reference sid-only sprint detail routes.
- [x] 3.3 Run targeted verification for Go sprint handlers/routes, Jest sprint workspace coverage, IM bridge client/command tests, and `openspec validate --specs`.
